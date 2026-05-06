// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"sync/atomic"

	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/geo"
	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmpbf"
)

// PBFSource is an offline [MapSource] backed by an OSM PBF file
// (typically a Geofabrik regional extract or the planet dump). The file
// is parsed once at startup and the relevant features are kept in RAM
// behind a coarse spatial grid so bbox queries are O(features in cell).
//
// Memory cost is roughly proportional to the PBF size:
//
//   - Aquitaine (~150 MB) → ~80 MB resident
//   - France    (~5 GB)  → ~1.5 GB resident
//   - Europe    (~28 GB) → ~7 GB resident
//
// Latency after load is in the millisecond range — no network at all.
type PBFSource struct {
	name     string
	bounds   geo.BBox
	features []data.Feature
	grid     map[pbfGridKey][]int
}

// PBFLoadStats is reported when a PBF finishes loading. Useful for
// printing a one-line summary to the user.
type PBFLoadStats struct {
	Path        string
	BytesRead   int64
	Features    int
	Nodes       int
	Ways        int
	Relations   int
	GridCells   int
	BoundingBox geo.BBox
}

// PBFProgress is the callback type used by [LoadPBF] to report progress
// while parsing. `read` is the number of bytes consumed so far, `total`
// is the size of the file. Progress is reported roughly every 8 MiB.
type PBFProgress func(read, total int64)

// pbfGridKey buckets features by 0.025-degree cells (≈ 2.5 km at the
// equator). Coarse enough that the grid stays small even for the planet,
// fine enough that a city-zoom query touches only a handful of cells.
type pbfGridKey struct{ x, y int32 }

// pbfGridCellSize controls bucket granularity in degrees. Lower values
// trade memory (more cells) for query speed (fewer features per cell).
const pbfGridCellSize = 0.025

// LoadPBF parses an OSM PBF file and returns a queryable [PBFSource].
// The progress callback is optional and may be called from the
// goroutines that paulmach/osmpbf spawns internally.
func LoadPBF(ctx context.Context, path string, onProgress PBFProgress) (*PBFSource, *PBFLoadStats, error) {
	f, err := os.Open(path) //nolint:gosec // path comes from user config
	if err != nil {
		return nil, nil, fmt.Errorf("open pbf: %w", err)
	}
	defer func() { _ = f.Close() }()

	st, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	totalBytes := st.Size()

	cr := &countingPBFReader{r: f}
	scanner := osmpbf.New(ctx, cr, runtime.NumCPU())
	defer func() { _ = scanner.Close() }()

	src := &PBFSource{
		name: path,
		grid: map[pbfGridKey][]int{},
	}
	stats := &PBFLoadStats{Path: path}

	// We need every node coordinate to reconstruct way geometries; the
	// PBF format stores node references by ID, not embedded points.
	// 8 bytes ID + 16 bytes lat/lng = 24 bytes per node. For a France
	// extract that's ~700 MB just for the lookup table — but it's
	// transient: dropped after Scan returns.
	nodeCoords := make(map[osm.NodeID][2]float64, 1<<20)

	var lastReport int64
	for scanner.Scan() {
		switch o := scanner.Object().(type) {
		case *osm.Node:
			stats.Nodes++
			nodeCoords[o.ID] = [2]float64{o.Lat, o.Lon}
			if isPOINode(o) {
				src.addFeature(nodeFeature(o))
			}
		case *osm.Way:
			stats.Ways++
			if !isRelevantWay(o) {
				continue
			}
			pts, ok := wayPoints(o, nodeCoords)
			if !ok || len(pts) < 2 {
				continue
			}
			src.addFeature(wayFeature(o, pts))
		case *osm.Relation:
			stats.Relations++
			// Multipolygon support is non-trivial — defer to a future
			// version. We render water/parks as ways for now.
		}

		if onProgress != nil {
			now := atomic.LoadInt64(&cr.bytes)
			if now-lastReport > 8*(1<<20) {
				onProgress(now, totalBytes)
				lastReport = now
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("pbf scan: %w", err)
	}

	stats.BytesRead = atomic.LoadInt64(&cr.bytes)
	stats.Features = len(src.features)
	stats.GridCells = len(src.grid)
	stats.BoundingBox = src.bounds
	if onProgress != nil {
		onProgress(stats.BytesRead, totalBytes)
	}
	return src, stats, nil
}

// FetchMapLayers implements [MapSource]. The query walks the grid cells
// the bbox touches and filters out features that are too detailed for
// the active zoom (same rule set the tile-based sources use).
func (p *PBFSource) FetchMapLayers(_ context.Context, bbox geo.BBox, zoom int) (data.FeatureCollection, error) {
	if p == nil {
		return data.FeatureCollection{}, errors.New("pbf: not loaded")
	}
	k1 := gridKeyFor(bbox.South, bbox.West)
	k2 := gridKeyFor(bbox.North, bbox.East)

	seen := make(map[int]struct{}, 256)
	var fc data.FeatureCollection
	for x := k1.x; x <= k2.x; x++ {
		for y := k1.y; y <= k2.y; y++ {
			for _, idx := range p.grid[pbfGridKey{x, y}] {
				if _, dup := seen[idx]; dup {
					continue
				}
				seen[idx] = struct{}{}
				f := p.features[idx]
				if !pbfFeatureVisible(f, zoom) {
					continue
				}
				fc.Features = append(fc.Features, f)
			}
		}
	}
	return fc, nil
}

// Bounds returns the bbox spanning every feature held by the source.
func (p *PBFSource) Bounds() geo.BBox { return p.bounds }

// Len returns the total feature count.
func (p *PBFSource) Len() int { return len(p.features) }

// addFeature registers a feature in both the linear store and the grid
// index. Out-of-band geometries are silently dropped.
func (p *PBFSource) addFeature(f data.Feature) {
	if f.Geometry.Kind == data.GeometryUnknown || len(f.Geometry.Points) == 0 {
		return
	}
	idx := len(p.features)
	p.features = append(p.features, f)

	bb := bboxOfPoints(f.Geometry.Points)
	if !p.bounds.Valid() || idx == 0 {
		p.bounds = bb
	} else {
		p.bounds = unionBBox(p.bounds, bb)
	}

	k1 := gridKeyFor(bb.South, bb.West)
	k2 := gridKeyFor(bb.North, bb.East)
	for x := k1.x; x <= k2.x; x++ {
		for y := k1.y; y <= k2.y; y++ {
			key := pbfGridKey{x, y}
			p.grid[key] = append(p.grid[key], idx)
		}
	}
}

func gridKeyFor(lat, lng float64) pbfGridKey {
	return pbfGridKey{
		x: int32(math.Floor(lng / pbfGridCellSize)),
		y: int32(math.Floor(lat / pbfGridCellSize)),
	}
}

func bboxOfPoints(pts []geo.LatLng) geo.BBox {
	bb := geo.BBox{
		South: pts[0].Lat,
		West:  pts[0].Lng,
		North: pts[0].Lat,
		East:  pts[0].Lng,
	}
	for _, p := range pts[1:] {
		if p.Lat < bb.South {
			bb.South = p.Lat
		}
		if p.Lat > bb.North {
			bb.North = p.Lat
		}
		if p.Lng < bb.West {
			bb.West = p.Lng
		}
		if p.Lng > bb.East {
			bb.East = p.Lng
		}
	}
	return bb
}

func unionBBox(a, b geo.BBox) geo.BBox {
	return geo.BBox{
		South: math.Min(a.South, b.South),
		West:  math.Min(a.West, b.West),
		North: math.Max(a.North, b.North),
		East:  math.Max(a.East, b.East),
	}
}

// isPOINode keeps only nodes that carry a tag CarTUI knows how to use.
// Filtering at parse time saves substantial memory.
func isPOINode(n *osm.Node) bool {
	for _, t := range n.Tags {
		switch t.Key {
		case "amenity", "shop", "tourism", "leisure", "place", "railway",
			"public_transport", "natural":
			return true
		}
	}
	return false
}

// isRelevantWay decides whether a way is worth keeping for rendering.
// Same set of tags the renderer's [data.OSMTags.Layer] understands.
func isRelevantWay(w *osm.Way) bool {
	for _, t := range w.Tags {
		switch t.Key {
		case "highway", "natural", "waterway", "landuse", "leisure",
			"building", "boundary", "place", "amenity", "shop",
			"tourism", "aeroway", "railway":
			return true
		}
	}
	return false
}

// nodeFeature converts an OSM node into a [data.Feature] with point
// geometry.
func nodeFeature(n *osm.Node) data.Feature {
	tags := make(data.OSMTags, len(n.Tags))
	for _, t := range n.Tags {
		tags[t.Key] = t.Value
	}
	return data.Feature{
		ID:       fmt.Sprintf("node/%d", n.ID),
		Tags:     tags,
		Name:     tags["name"],
		Geometry: data.Geometry{Kind: data.GeometryPoint, Points: []geo.LatLng{{Lat: n.Lat, Lng: n.Lon}}},
	}
}

// wayFeature converts an OSM way into a Feature with the appropriate
// geometry kind (LineString or Polygon for closed area features).
func wayFeature(w *osm.Way, pts []geo.LatLng) data.Feature {
	tags := make(data.OSMTags, len(w.Tags))
	for _, t := range w.Tags {
		tags[t.Key] = t.Value
	}
	kind := data.GeometryLineString
	if isAreaWay(tags, pts) {
		kind = data.GeometryPolygon
	}
	return data.Feature{
		ID:       fmt.Sprintf("way/%d", w.ID),
		Tags:     tags,
		Name:     tags["name"],
		Geometry: data.Geometry{Kind: kind, Points: pts},
	}
}

// isAreaWay returns true when a closed way represents an area (water,
// park, building) rather than a line (road, river bank).
func isAreaWay(t data.OSMTags, pts []geo.LatLng) bool {
	if len(pts) < 4 || pts[0] != pts[len(pts)-1] {
		return false
	}
	if t.IsBuilding() || t.IsWater() || t.IsGreen() {
		return true
	}
	if t.Has("amenity") || t.Has("leisure") || t.Has("tourism") || t.Has("shop") {
		return true
	}
	if t.Has("aeroway") {
		return true
	}
	return false
}

// wayPoints reconstructs a way geometry from its node references by
// looking up coordinates in the prebuilt index. Returns false when any
// referenced node was missing — typical for ways that cross the PBF
// extract boundary.
func wayPoints(w *osm.Way, nodes map[osm.NodeID][2]float64) ([]geo.LatLng, bool) {
	if len(w.Nodes) < 2 {
		return nil, false
	}
	pts := make([]geo.LatLng, 0, len(w.Nodes))
	for _, n := range w.Nodes {
		ll, ok := nodes[n.ID]
		if !ok {
			return nil, false
		}
		pts = append(pts, geo.LatLng{Lat: ll[0], Lng: ll[1]})
	}
	return pts, true
}

// pbfFeatureVisible applies the same zoom-aware tier rules used by the
// tile-based sources, but expressed directly against OSM tags (which
// are the canonical form — the tile rewriters convert *to* this).
func pbfFeatureVisible(f data.Feature, zoom int) bool {
	t := f.Tags
	if t.IsBuilding() && zoom < 14 {
		return false
	}
	if t.Has("amenity") || t.Has("shop") || t.Has("tourism") {
		// POI-like: hidden at low zoom.
		if zoom < 12 && f.Geometry.Kind == data.GeometryPoint {
			return false
		}
	}
	switch t.Road() {
	case data.RoadResidential:
		if zoom < 13 {
			return false
		}
	case data.RoadSecondary:
		if zoom < 11 {
			return false
		}
	}
	if t.Has("place") && f.Geometry.Kind == data.GeometryPoint {
		switch t.Get("place") {
		case "country", "state", "region":
			return true
		case "city", "town":
			return zoom >= 7
		case "village", "suburb":
			return zoom >= 11
		case "neighbourhood", "hamlet", "locality":
			return zoom >= 13
		}
	}
	return true
}

// countingPBFReader wraps an io.Reader and tracks total bytes read so
// the loader can estimate progress. The counter is updated atomically
// because paulmach/osmpbf parses with a goroutine pool.
type countingPBFReader struct {
	r     io.Reader
	bytes int64
}

func (c *countingPBFReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	atomic.AddInt64(&c.bytes, int64(n))
	return n, err
}

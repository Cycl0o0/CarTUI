// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package geo

import "math"

// Haversine returns the great-circle distance between two coordinates, in
// metres, modelling the Earth as a sphere of radius [EarthRadiusMeters].
//
// The function is fast and accurate to roughly 0.5% — sufficient for any
// in-app distance display. For survey-grade results, prefer [Vincenty].
func Haversine(a, b LatLng) float64 {
	const deg2rad = math.Pi / 180
	lat1 := a.Lat * deg2rad
	lat2 := b.Lat * deg2rad
	dlat := (b.Lat - a.Lat) * deg2rad
	dlng := (b.Lng - a.Lng) * deg2rad

	h := math.Sin(dlat/2)*math.Sin(dlat/2) +
		math.Cos(lat1)*math.Cos(lat2)*
			math.Sin(dlng/2)*math.Sin(dlng/2)
	return 2 * EarthRadiusMeters * math.Asin(math.Sqrt(h))
}

// WGS84 ellipsoid parameters used by [Vincenty].
const (
	wgs84A = 6378137.0         // semi-major axis (m)
	wgs84B = 6356752.314245    // semi-minor axis (m)
	wgs84F = 1 / 298.257223563 // flattening
)

// Vincenty returns the geodesic distance between two coordinates, in metres,
// computed on the WGS84 ellipsoid. Accurate to a few millimetres but
// iterative — about 10× slower than [Haversine].
//
// Returns 0 for identical points and falls back to [Haversine] if the
// iteration fails to converge (e.g. for nearly antipodal pairs).
func Vincenty(p1, p2 LatLng) float64 {
	if p1 == p2 {
		return 0
	}
	const deg2rad = math.Pi / 180
	const tol = 1e-12
	const maxIter = 200

	L := (p2.Lng - p1.Lng) * deg2rad
	U1 := math.Atan((1 - wgs84F) * math.Tan(p1.Lat*deg2rad))
	U2 := math.Atan((1 - wgs84F) * math.Tan(p2.Lat*deg2rad))
	sinU1, cosU1 := math.Sin(U1), math.Cos(U1)
	sinU2, cosU2 := math.Sin(U2), math.Cos(U2)

	lambda := L
	var (
		sinLambda, cosLambda      float64
		sinSigma, cosSigma, sigma float64
		sinAlpha, cos2Alpha       float64
		cos2SigmaM                float64
	)
	for i := 0; i < maxIter; i++ {
		sinLambda, cosLambda = math.Sin(lambda), math.Cos(lambda)
		sinSigma = math.Sqrt(
			(cosU2*sinLambda)*(cosU2*sinLambda) +
				(cosU1*sinU2-sinU1*cosU2*cosLambda)*
					(cosU1*sinU2-sinU1*cosU2*cosLambda),
		)
		if sinSigma == 0 {
			return 0
		}
		cosSigma = sinU1*sinU2 + cosU1*cosU2*cosLambda
		sigma = math.Atan2(sinSigma, cosSigma)
		sinAlpha = cosU1 * cosU2 * sinLambda / sinSigma
		cos2Alpha = 1 - sinAlpha*sinAlpha
		if cos2Alpha == 0 {
			cos2SigmaM = 0
		} else {
			cos2SigmaM = cosSigma - 2*sinU1*sinU2/cos2Alpha
		}
		C := wgs84F / 16 * cos2Alpha * (4 + wgs84F*(4-3*cos2Alpha))
		prev := lambda
		lambda = L + (1-C)*wgs84F*sinAlpha*
			(sigma+C*sinSigma*(cos2SigmaM+C*cosSigma*(-1+2*cos2SigmaM*cos2SigmaM)))
		if math.Abs(lambda-prev) < tol {
			uSq := cos2Alpha * (wgs84A*wgs84A - wgs84B*wgs84B) / (wgs84B * wgs84B)
			A := 1 + uSq/16384*(4096+uSq*(-768+uSq*(320-175*uSq)))
			B := uSq / 1024 * (256 + uSq*(-128+uSq*(74-47*uSq)))
			deltaSigma := B * sinSigma * (cos2SigmaM + B/4*(cosSigma*(-1+2*cos2SigmaM*cos2SigmaM)-
				B/6*cos2SigmaM*(-3+4*sinSigma*sinSigma)*(-3+4*cos2SigmaM*cos2SigmaM)))
			return wgs84B * A * (sigma - deltaSigma)
		}
	}
	// Failed to converge (rare; near-antipodal pairs). Fall back to
	// haversine, which is well-defined everywhere.
	return Haversine(p1, p2)
}

// PathLength returns the total Haversine length of a sequence of coordinates,
// in metres. Returns 0 for fewer than two points.
func PathLength(points []LatLng) float64 {
	if len(points) < 2 {
		return 0
	}
	var total float64
	for i := 1; i < len(points); i++ {
		total += Haversine(points[i-1], points[i])
	}
	return total
}

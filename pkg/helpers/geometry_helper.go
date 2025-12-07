package helpers

import "math"

func CalculatePolygonArea(polygon [][][]float64) float64 {
	if len(polygon) == 0 {
		return 0
	}

	// Calculate signed area of exterior ring (first ring)
	// Counter-clockwise = positive, Clockwise = negative
	exteriorSignedArea := calculateRingSignedArea(polygon[0])

	// Exterior ring: always add absolute value (area should be positive)
	exteriorArea := math.Abs(exteriorSignedArea)

	// Subtract area of interior rings (holes)
	// Interior rings should be clockwise (negative), but we always subtract absolute value
	for i := 1; i < len(polygon); i++ {
		interiorSignedArea := calculateRingSignedArea(polygon[i])
		// Always subtract absolute value of interior rings (they are holes)
		exteriorArea -= math.Abs(interiorSignedArea)
	}

	// Ensure non-negative result
	if exteriorArea < 0 {
		return 0
	}

	return exteriorArea
}

func calculateRingSignedArea(ring [][]float64) float64 {
	if len(ring) < 3 {
		return 0 // Need at least 3 points to form a polygon
	}

	var area float64
	n := len(ring)

	// Shoelace formula: area = 0.5 * Σ(xi * yi+1 - xi+1 * yi)
	// Positive = counter-clockwise (ngược chiều kim đồng hồ)
	// Negative = clockwise (cùng chiều kim đồng hồ)
	for i := 0; i < n; i++ {
		j := (i + 1) % n // Next point (wraps around to first point)

		// Ensure we have at least 2 coordinates (x, y)
		if len(ring[i]) < 2 || len(ring[j]) < 2 {
			continue
		}

		xi := ring[i][0] // longitude or x
		yi := ring[i][1] // latitude or y
		xj := ring[j][0]
		yj := ring[j][1]

		area += xi*yj - xj*yi
	}

	return 0.5 * area // Return signed area (not absolute value)
}

// CalculatePolygonAreaInSquareMeters calculates polygon area and converts to square meters
// This is a simplified conversion assuming coordinates are in degrees (WGS84)
// For production use, consider using a proper projection library like PROJ
//
// Note: This uses a simple approximation. For accurate results, use proper coordinate transformation
func CalculatePolygonAreaInSquareMeters(polygon [][][]float64) float64 {
	areaInSquareDegrees := CalculatePolygonArea(polygon)

	// Approximate conversion: 1 degree latitude ≈ 111,320 meters
	// 1 degree longitude varies by latitude, but we'll use an average
	// This is a simplified calculation - for accurate results, use proper projection
	averageLatitude := getAverageLatitude(polygon)

	// Convert square degrees to square meters
	// 1 degree latitude = 111,320 meters
	// 1 degree longitude at average latitude = 111,320 * cos(latitude) meters
	latMeters := 111320.0
	lonMeters := 111320.0 * math.Cos(averageLatitude*math.Pi/180.0)

	return areaInSquareDegrees * latMeters * lonMeters
}

// getAverageLatitude calculates the average latitude of all points in the polygon
func getAverageLatitude(polygon [][][]float64) float64 {
	if len(polygon) == 0 || len(polygon[0]) == 0 {
		return 0
	}

	var sumLat float64
	var count int

	for _, ring := range polygon {
		for _, point := range ring {
			if len(point) >= 2 {
				sumLat += point[1] // latitude is typically the second coordinate
				count++
			}
		}
	}

	if count == 0 {
		return 0
	}

	return sumLat / float64(count)
}

// ValidatePolygon validates that a polygon has valid structure
// Returns true if polygon is valid, false otherwise
func ValidatePolygon(polygon [][][]float64) bool {
	if len(polygon) == 0 {
		return false
	}

	// Check exterior ring (first ring)
	if len(polygon[0]) < 3 {
		return false // Need at least 3 points
	}

	// Check that all points have at least 2 coordinates
	for _, ring := range polygon {
		for _, point := range ring {
			if len(point) < 2 {
				return false
			}
		}
	}

	return true
}

func IsPointInPolygon(lat, lon float64, polygon [][][]float64) bool {
	if len(polygon) == 0 {
		return false
	}

	// First check: point must be inside the exterior ring (first ring)
	if !isPointInRing(lon, lat, polygon[0]) {
		return false
	}

	// Second check: point must NOT be inside any interior ring (holes)
	for i := 1; i < len(polygon); i++ {
		if isPointInRing(lon, lat, polygon[i]) {
			return false // Point is inside a hole, so it's outside the polygon
		}
	}

	return true
}

// isPointInRing checks if a point is inside a single ring using Ray Casting Algorithm
// Casts a horizontal ray from point (px, py) to the right and counts edge intersections
func isPointInRing(px, py float64, ring [][]float64) bool {
	if len(ring) < 3 {
		return false // Need at least 3 points to form a polygon
	}

	inside := false
	n := len(ring)

	for i := range n {
		j := (i + 1) % n // Next point (wraps around to first point)

		if len(ring[i]) < 2 || len(ring[j]) < 2 {
			continue
		}

		xi := ring[i][0] // longitude or x
		yi := ring[i][1] // latitude or y
		xj := ring[j][0]
		yj := ring[j][1]

		if (yi > py) != (yj > py) {
			if yj != yi {
				intersectX := xi + (py-yi)*(xj-xi)/(yj-yi)
				if px < intersectX {
					inside = !inside // Toggle inside state
				}
			}
		}
	}

	return inside
}

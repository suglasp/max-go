// geometry.go : provides integer grid coordinate operations.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package game

// Point is an integer grid coordinate, used for map cells or pixel offsets.
//
// C++ ported : point.hpp - Point
type Point struct {
	X, Y int
}

// Add returns p+q.
func (p Point) Add(q Point) Point {
	return Point{p.X + q.X, p.Y + q.Y}
}

// Sub returns p-q.
func (p Point) Sub(q Point) Point {
	return Point{p.X - q.X, p.Y - q.Y}
}

// Equal reports whether p and q are the same cell.
func (p Point) Equal(q Point) bool {
	return p.X == q.X && p.Y == q.Y
}

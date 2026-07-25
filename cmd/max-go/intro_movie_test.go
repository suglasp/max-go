// intro_movie_test.go : tests the saved and command line controls for the intro movie.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import "testing"

// TestIntroMovieEnabled covers the boot gate: the saved game.intro_movie
// preference suppresses the movie like a persistent -nomovie, while -skipintro,
// -nomovie and the quick start jumps keep their existing suppression.
func TestIntroMovieEnabled(t *testing.T) {
	cases := []struct {
		name                                    string
		skipIntro, noMovie, introMovie, quickSt bool
		want                                    bool
	}{
		{"normal boot plays", false, false, true, false, true},
		{"intro_movie off suppresses", false, false, false, false, false},
		{"nomovie flag wins over setting", false, true, true, false, false},
		{"skipintro suppresses", true, false, true, false, false},
		{"quick start shows only splash", false, false, true, true, false},
		{"all off", true, true, false, true, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := introMovieEnabled(c.skipIntro, c.noMovie, c.introMovie, c.quickSt); got != c.want {
				t.Errorf("introMovieEnabled(skip=%v nomovie=%v intro=%v quick=%v) = %v, want %v",
					c.skipIntro, c.noMovie, c.introMovie, c.quickSt, got, c.want)
			}
		})
	}
}

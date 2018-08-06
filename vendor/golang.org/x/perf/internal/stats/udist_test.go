// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stats

import (
	"fmt"
	"math"
	"testing"
)

func aeqTable(a, b [][]float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			// "%f" precision
			if math.Abs(a[i][j]-b[i][j]) >= 0.000001 {
				return false
			}
		}
	}
	return true
}

// U distribution for N=3 up to U=5.
var udist3 = [][]float64{
	//    m=1         2         3
	{0.250000, 0.100000, 0.050000}, // U=0
	{0.500000, 0.200000, 0.100000}, // U=1
	{0.750000, 0.400000, 0.200000}, // U=2
	{1.000000, 0.600000, 0.350000}, // U=3
	{1.000000, 0.800000, 0.500000}, // U=4
	{1.000000, 0.900000, 0.650000}, // U=5
}

// U distribution for N=5 up to U=5.
var udist5 = [][]float64{
	//    m=1         2         3         4         5
	{0.166667, 0.047619, 0.017857, 0.007937, 0.003968}, // U=0
	{0.333333, 0.095238, 0.035714, 0.015873, 0.007937}, // U=1
	{0.500000, 0.190476, 0.071429, 0.031746, 0.015873}, // U=2
	{0.666667, 0.285714, 0.125000, 0.055556, 0.027778}, // U=3
	{0.833333, 0.428571, 0.196429, 0.095238, 0.047619}, // U=4
	{1.000000, 0.571429, 0.285714, 0.142857, 0.075397}, // U=5
}

func TestUDist(t *testing.T) {
	makeTable := func(n int) [][]float64 {
		out := make([][]float64, 6)
		for U := 0; U < 6; U++ {
			out[U] = make([]float64, n)
			for m := 1; m <= n; m++ {
				out[U][m-1] = UDist{N1: m, N2: n}.CDF(float64(U))
			}
		}
		return out
	}
	fmtTable := func(a [][]float64) string {
		out := fmt.Sprintf("%8s", "m=")
		for m := 1; m <= len(a[0]); m++ {
			out += fmt.Sprintf("%9d", m)
		}
		out += "\n"

		for U, row := range a {
			out += fmt.Sprintf("U=%-6d", U)
			for m := 1; m <= len(a[0]); m++ {
				out += fmt.Sprintf(" %f", row[m-1])
			}
			out += "\n"
		}
		return out
	}

	// Compare against tables given in Mann, Whitney (1947).
	got3 := makeTable(3)
	if !aeqTable(got3, udist3) {
		t.Errorf("For n=3, want:\n%sgot:\n%s", fmtTable(udist3), fmtTable(got3))
	}

	got5 := makeTable(5)
	if !aeqTable(got5, udist5) {
		t.Errorf("For n=5, want:\n%sgot:\n%s", fmtTable(udist5), fmtTable(got5))
	}
}

func BenchmarkUDist(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// R uses the exact distribution up to N=50.
		// N*M/2=1250 is the hardest point to get the CDF for.
		UDist{N1: 50, N2: 50}.CDF(1250)
	}
}

func TestUDistTies(t *testing.T) {
	makeTable := func(m, N int, t []int, minx, maxx float64) [][]float64 {
		out := [][]float64{}
		dist := UDist{N1: m, N2: N - m, T: t}
		for x := minx; x <= maxx; x += 0.5 {
			// Convert x from uQt' to uQv'.
			U := x - float64(m*m)/2
			P := dist.CDF(U)
			if len(out) == 0 || !aeq(out[len(out)-1][1], P) {
				out = append(out, []float64{x, P})
			}
		}
		return out
	}
	fmtTable := func(table [][]float64) string {
		out := ""
		for _, row := range table {
			out += fmt.Sprintf("%5.1f %f\n", row[0], row[1])
		}
		return out
	}

	// Compare against Table 1 from Klotz (1966).
	got := makeTable(5, 10, []int{1, 1, 2, 1, 1, 2, 1, 1}, 12.5, 19.5)
	want := [][]float64{
		{12.5, 0.003968}, {13.5, 0.007937},
		{15.0, 0.023810}, {16.5, 0.047619},
		{17.5, 0.071429}, {18.0, 0.087302},
		{19.0, 0.134921}, {19.5, 0.138889},
	}
	if !aeqTable(got, want) {
		t.Errorf("Want:\n%sgot:\n%s", fmtTable(want), fmtTable(got))
	}

	got = makeTable(10, 21, []int{6, 5, 4, 3, 2, 1}, 52, 87)
	want = [][]float64{
		{52.0, 0.000014}, {56.5, 0.000128},
		{57.5, 0.000145}, {60.0, 0.000230},
		{61.0, 0.000400}, {62.0, 0.000740},
		{62.5, 0.000797}, {64.0, 0.000825},
		{64.5, 0.001165}, {65.5, 0.001477},
		{66.5, 0.002498}, {67.0, 0.002725},
		{67.5, 0.002895}, {68.0, 0.003150},
		{68.5, 0.003263}, {69.0, 0.003518},
		{69.5, 0.003603}, {70.0, 0.005648},
		{70.5, 0.005818}, {71.0, 0.006626},
		{71.5, 0.006796}, {72.0, 0.008157},
		{72.5, 0.009688}, {73.0, 0.009801},
		{73.5, 0.010430}, {74.0, 0.011111},
		{74.5, 0.014230}, {75.0, 0.014612},
		{75.5, 0.017249}, {76.0, 0.018307},
		{76.5, 0.020178}, {77.0, 0.022270},
		{77.5, 0.023189}, {78.0, 0.026931},
		{78.5, 0.028207}, {79.0, 0.029979},
		{79.5, 0.030931}, {80.0, 0.038969},
		{80.5, 0.043063}, {81.0, 0.044262},
		{81.5, 0.046389}, {82.0, 0.049581},
		{82.5, 0.056300}, {83.0, 0.058027},
		{83.5, 0.063669}, {84.0, 0.067454},
		{84.5, 0.074122}, {85.0, 0.077425},
		{85.5, 0.083498}, {86.0, 0.094079},
		{86.5, 0.096693}, {87.0, 0.101132},
	}
	if !aeqTable(got, want) {
		t.Errorf("Want:\n%sgot:\n%s", fmtTable(want), fmtTable(got))
	}

	got = makeTable(8, 16, []int{2, 2, 2, 2, 2, 2, 2, 2}, 32, 54)
	want = [][]float64{
		{32.0, 0.000078}, {34.0, 0.000389},
		{36.0, 0.001088}, {38.0, 0.002642},
		{40.0, 0.005905}, {42.0, 0.011500},
		{44.0, 0.021057}, {46.0, 0.035664},
		{48.0, 0.057187}, {50.0, 0.086713},
		{52.0, 0.126263}, {54.0, 0.175369},
	}
	if !aeqTable(got, want) {
		t.Errorf("Want:\n%sgot:\n%s", fmtTable(want), fmtTable(got))
	}

	// Check remaining tables from Klotz against the reference
	// implementation.
	checkRef := func(n1 int, tie []int) {
		wantPMF1, wantCDF1 := udistRef(n1, tie)

		dist := UDist{N1: n1, N2: sumint(tie) - n1, T: tie}
		gotPMF, wantPMF := [][]float64{}, [][]float64{}
		gotCDF, wantCDF := [][]float64{}, [][]float64{}
		N := sumint(tie)
		for U := 0.0; U <= float64(n1*(N-n1)); U += 0.5 {
			gotPMF = append(gotPMF, []float64{U, dist.PMF(U)})
			gotCDF = append(gotCDF, []float64{U, dist.CDF(U)})
			wantPMF = append(wantPMF, []float64{U, wantPMF1[int(U*2)]})
			wantCDF = append(wantCDF, []float64{U, wantCDF1[int(U*2)]})
		}
		if !aeqTable(wantPMF, gotPMF) {
			t.Errorf("For PMF of n1=%v, t=%v, want:\n%sgot:\n%s", n1, tie, fmtTable(wantPMF), fmtTable(gotPMF))
		}
		if !aeqTable(wantCDF, gotCDF) {
			t.Errorf("For CDF of n1=%v, t=%v, want:\n%sgot:\n%s", n1, tie, fmtTable(wantCDF), fmtTable(gotCDF))
		}
	}
	checkRef(5, []int{1, 1, 2, 1, 1, 2, 1, 1})
	checkRef(5, []int{1, 1, 2, 1, 1, 1, 2, 1})
	checkRef(5, []int{1, 3, 1, 2, 1, 1, 1})
	checkRef(8, []int{1, 2, 1, 1, 1, 1, 2, 2, 1, 2})
	checkRef(12, []int{3, 3, 4, 3, 4, 5})
	checkRef(10, []int{1, 2, 3, 4, 5, 6})
}

func BenchmarkUDistTies(b *testing.B) {
	// Worst case: just one tie.
	n := 20
	t := make([]int, 2*n-1)
	for i := range t {
		t[i] = 1
	}
	t[0] = 2

	for i := 0; i < b.N; i++ {
		UDist{N1: n, N2: n, T: t}.CDF(float64(n*n) / 2)
	}
}

func XTestPrintUmemo(t *testing.T) {
	// Reproduce table from Cheung, Klotz.
	ties := []int{4, 5, 3, 4, 6}
	printUmemo(makeUmemo(80, 10, ties), ties)
}

// udistRef computes the PMF and CDF of the U distribution for two
// samples of sizes n1 and sum(t)-n1 with tie vector t. The returned
// pmf and cdf are indexed by 2*U.
//
// This uses the "graphical method" of Klotz (1966). It is very slow
// (Θ(∏ (t[i]+1)) = Ω(2^|t|)), but very correct, and hence useful as a
// reference for testing faster implementations.
func udistRef(n1 int, t []int) (pmf, cdf []float64) {
	// Enumerate all u vectors for which 0 <= u_i <= t_i. Count
	// the number of permutations of two samples of sizes n1 and
	// sum(t)-n1 with tie vector t and accumulate these counts by
	// their U statistics in count[2*U].
	counts := make([]int, 1+2*n1*(sumint(t)-n1))

	u := make([]int, len(t))
	u[0] = -1 // Get enumeration started.
enumu:
	for {
		// Compute the next u vector.
		u[0]++
		for i := 0; i < len(u) && u[i] > t[i]; i++ {
			if i == len(u)-1 {
				// All u vectors have been enumerated.
				break enumu
			}
			// Carry.
			u[i+1]++
			u[i] = 0
		}

		// Is this a legal u vector?
		if sumint(u) != n1 {
			// Klotz (1966) has a method for directly
			// enumerating legal u vectors, but the point
			// of this is to be correct, not fast.
			continue
		}

		// Compute 2*U statistic for this u vector.
		twoU, vsum := 0, 0
		for i, u_i := range u {
			v_i := t[i] - u_i
			// U = U + vsum*u_i + u_i*v_i/2
			twoU += 2*vsum*u_i + u_i*v_i
			vsum += v_i
		}

		// Compute Π choose(t_i, u_i). This is the number of
		// ways of permuting the input sample under u.
		prod := 1
		for i, u_i := range u {
			prod *= int(mathChoose(t[i], u_i) + 0.5)
		}

		// Accumulate the permutations on this u path.
		counts[twoU] += prod

		if false {
			// Print a table in the form of Klotz's
			// "direct enumeration" example.
			//
			// Convert 2U = 2UQV' to UQt' used in Klotz
			// examples.
			UQt := float64(twoU)/2 + float64(n1*n1)/2
			fmt.Printf("%+v %f %-2d\n", u, UQt, prod)
		}
	}

	// Convert counts into probabilities for PMF and CDF.
	pmf = make([]float64, len(counts))
	cdf = make([]float64, len(counts))
	total := int(mathChoose(sumint(t), n1) + 0.5)
	for i, count := range counts {
		pmf[i] = float64(count) / float64(total)
		if i > 0 {
			cdf[i] = cdf[i-1]
		}
		cdf[i] += pmf[i]
	}
	return
}

// printUmemo prints the output of makeUmemo for debugging.
func printUmemo(A []map[ukey]float64, t []int) {
	fmt.Printf("K\tn1\t2*U\tpr\n")
	for K := len(A) - 1; K >= 0; K-- {
		for i, pr := range A[K] {
			_, ref := udistRef(i.n1, t[:K])
			fmt.Printf("%v\t%v\t%v\t%v\t%v\n", K, i.n1, i.twoU, pr, ref[i.twoU])
		}
	}
}

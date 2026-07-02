//go:build !darwin

package topdown

func fixupDarwinGo118(x string, _ string) string {
	return x
}

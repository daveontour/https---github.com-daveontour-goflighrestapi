package globals

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"flightresourcerestapi/models"
)

func Contains(elems []string, v string) bool {
	for _, s := range elems {
		if v == s {
			return true
		}
	}
	return false
}
func CleanJSON(sb strings.Builder) string {

	s := sb.String()
	if last := len(s) - 1; last >= 0 && s[last] == ',' {
		s = s[:last]
	}

	s = s + "}"

	return s
}

func GetUserProfiles() []models.UserProfile {

	var users models.Users
	if err := UserViper.Unmarshal(&users); err != nil {
		return nil
	}

	return users.Users
}
func Max(x, y int) int {
	if x < y {
		return y
	}
	return x
}
func Min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func ExePath() (string, error) {
	prog := os.Args[0]
	p, err := filepath.Abs(prog)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(p)
	if err == nil {
		if !fi.Mode().IsDir() {
			return p, nil
		}
		err = fmt.Errorf("%s is directory", p)
	}
	if filepath.Ext(p) == "" {
		p += ".exe"
		fi, err := os.Stat(p)
		if err == nil {
			if !fi.Mode().IsDir() {
				return p, nil
			}
			err = fmt.Errorf("%s is directory", p)
		}
	}
	return "", err
}

func ExeTime(name string) func() {
	start := time.Now()
	return func() {
		MetricsLogger.Info(fmt.Sprintf("%s execution time: %v", name, time.Since(start)))
	}
}

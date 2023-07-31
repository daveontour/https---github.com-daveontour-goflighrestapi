package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func contains(elems []string, v string) bool {
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
func readBytesFromFile(filename string) (byteResult []byte) {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)

	fileContent, err := os.Open(filepath.Join(exPath, filename))

	if err != nil {
		logger.Fatal(err)
		return
	}

	defer fileContent.Close()

	byteResult, _ = ioutil.ReadAll(fileContent)

	return
}

func getUserProfiles() []UserProfile {

	//Read read the file for each request so changes can be made without the need to restart the server

	// ex, err := os.Executable()
	// if err != nil {
	// 	panic(err)
	// }
	// exPath := filepath.Dir(ex)

	// fileContent, err := os.Open(filepath.Join(exPath, "users.json"))

	// if err != nil {
	// 	logger.Error("Error Reading " + filepath.Join(exPath, "users.json"))
	// 	logger.Error(err.Error())
	// 	return []UserProfile{}
	// }

	// defer fileContent.Close()

	// byteResult, _ := ioutil.ReadAll(fileContent)

	var users Users

	json.Unmarshal([]byte(readBytesFromFile("users.json")), &users)

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

func exePath() (string, error) {
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

func exeTime(name string) func() {
	start := time.Now()
	return func() {
		metricsLogger.Info(fmt.Sprintf("%s execution time: %v", name, time.Since(start)))
	}
}

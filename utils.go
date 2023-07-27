package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
)

func contains(elems []string, v string) bool {
	for _, s := range elems {
		if v == s {
			return true
		}
	}
	return false
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

package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func startGinServer() {

	mode := gin.ReleaseMode
	if serviceConfig.DebugService {
		mode = gin.DebugMode
	}
	gin.SetMode(mode)

	router := gin.New()

	// Configure all the endpoints for the HTTP Server

	// Test purposes only to just printout whatever was received by the server
	if serviceConfig.TestHTTPServer {
		router.POST("/test", testQuery)
	}
	router.GET("/reinit/:apt", reinit)
	router.GET("/getFlights/:apt", getRequestedFlightsAPI)
	router.GET("/stopJobs/:apt/:userToken", stopJobs)
	router.GET("/stopAllAptJobs/:apt", stopAllAptJobs)
	router.GET("/rescheduleAllAptJobs/:apt", rescheduleAllAptJobs)
	router.GET("/getResources/:apt", getResourceAPI)
	router.GET("/getConfiguredResources/:apt/:resourceType", getConfiguredResources)
	router.GET("/getConfiguredResources/:apt", getConfiguredResources)
	router.GET("/help", func(c *gin.Context) {
		data, err := os.ReadFile("./help.html")
		if err != nil {
			return
		}
		c.Header("Content-Type", "text/html")
		_, _ = c.Writer.Write(data)
	})

	// Start it up with the configured security mode
	if !serviceConfig.UseHTTPS {

		err := router.Run(serviceConfig.ServiceIPPort)
		if err != nil {
			logger.Fatal("Unable to start HTTP server.")
			wg.Done()
			os.Exit(2)
		}

	} else if serviceConfig.KeyFile != "" && serviceConfig.CertFile != "" {

		server := http.Server{Addr: serviceConfig.ServiceIPPort, Handler: router}
		err := server.ListenAndServeTLS(serviceConfig.CertFile, serviceConfig.KeyFile)
		if err != nil {
			logger.Fatal("Unable to start HTTPS server. Likely cause is that the keyFile or certFile were not found")
			wg.Done()
			os.Exit(2)
		}

	} else {

		cert := &x509.Certificate{
			SerialNumber: big.NewInt(1658),
			Subject: pkix.Name{
				Organization:  []string{"ORGANIZATION_NAME"},
				Country:       []string{"COUNTRY_CODE"},
				Province:      []string{"PROVINCE"},
				Locality:      []string{"CITY"},
				StreetAddress: []string{"ADDRESS"},
				PostalCode:    []string{"POSTAL_CODE"},
			},
			NotBefore:    time.Now(),
			NotAfter:     time.Now().AddDate(10, 0, 0),
			SubjectKeyId: []byte{1, 2, 3, 4, 6},
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
			KeyUsage:     x509.KeyUsageDigitalSignature,
		}
		priv, _ := rsa.GenerateKey(rand.Reader, 2048)
		pub := &priv.PublicKey

		// Sign the certificate
		certificate, _ := x509.CreateCertificate(rand.Reader, cert, cert, pub, priv)

		certBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate})
		keyBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

		// Generate a key pair from your pem-encoded cert and key ([]byte).
		x509Cert, _ := tls.X509KeyPair(certBytes, keyBytes)

		tlsConfig := &tls.Config{Certificates: []tls.Certificate{x509Cert}}
		server := http.Server{Addr: serviceConfig.ServiceIPPort, Handler: router, TLSConfig: tlsConfig}

		err := server.ListenAndServeTLS("", "")
		if err != nil {
			logger.Fatal("Unable to start HTTPS server with local certificates and key")
			wg.Done()
			os.Exit(2)
		}
	}

}

func reinit(c *gin.Context) {
	// Get the profile of the user making the request
	userProfile := getUserProfile(c, "")
	requestLogger.Info(fmt.Sprintf("RestAPI request for user '%s' at %s : %s", userProfile.UserName, c.RemoteIP(), c.Request.RequestURI)) // Create the response object so we can return early if required

	apt := c.Param("apt")
	reInitAirport(apt)
}

func stopJobs(c *gin.Context) {
	// Get the profile of the user making the request
	userProfile := getUserProfile(c, "")
	requestLogger.Info(fmt.Sprintf("RestAPI request for user '%s' at %s: %s", userProfile.UserName, c.RemoteIP(), c.Request.RequestURI)) // Create the response object so we can return early if required

	apt := c.Param("apt")
	userToken := c.Param("userToken")
	s := schedulerMap[apt]
	s.RemoveByTag(userToken)
	logger.Info(fmt.Sprintf("All Aiport Jobs Stopped for %s, user %s", apt, userToken))
}

func stopAllAptJobs(c *gin.Context) {
	// Get the profile of the user making the request
	userProfile := getUserProfile(c, "")
	requestLogger.Info(fmt.Sprintf("RestAPI request for user '%s' at %s: %s", userProfile.UserName, c.RemoteIP(), c.Request.RequestURI)) // Create the response object so we can return early if required

	apt := c.Param("apt")

	// Get the schedule for the particular airport and clear it
	s := schedulerMap[apt]
	s.Clear()
	logger.Info(fmt.Sprintf("All Aiport Jobs Stopped for %s", apt))
}
func rescheduleAllAptJobs(c *gin.Context) {
	// Get the profile of the user making the request
	userProfile := getUserProfile(c, "")
	requestLogger.Info(fmt.Sprintf("RestAPI request for user '%s' at %s: %s", userProfile.UserName, c.RemoteIP(), c.Request.RequestURI)) // Create the response object so we can return early if required

	apt := c.Param("apt")

	// Reload the schdule of jobs for the airport
	reloadschedulePushes(apt)
	logger.Info(fmt.Sprintf("Rescheduled All Aiport Jobs Stopped for %s", apt))
}

func getUserProfile(c *gin.Context, userToken string) UserProfile {

	//Read read the file for each request so changes can be made without the need to restart the server

	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)

	fileContent, err := os.Open(filepath.Join(exPath, "users.json"))

	if err != nil {
		logger.Error("Error Reading " + filepath.Join(exPath, "users.json"))
		logger.Error(err.Error())
		return UserProfile{}
	}

	defer fileContent.Close()

	byteResult, _ := ioutil.ReadAll(fileContent)

	var users Users

	json.Unmarshal([]byte(byteResult), &users)

	key := userToken

	if c != nil {
		keys := c.Request.Header["token"]
		key = "default"

		if keys != nil {
			key = keys[0]
		}

	}
	userProfile := UserProfile{}

	for _, u := range users.Users {
		if key == u.Key {
			userProfile = u
			break
		}
	}

	return userProfile
}

// Function to just write what was recieved by the server
func testQuery(c *gin.Context) {
	if logger.Level == logrus.TraceLevel {
		logger.Info("Received message on test HTTP Server")
		jsonData, _ := ioutil.ReadAll(c.Request.Body)
		fmt.Println(string(jsonData[:]))
	} else {
		logger.Info("Received message on test HTTP Server")
	}
}

type GetFlightsError struct {
	StatusCode int
	Err        error
}

func (r *GetFlightsError) Error() string {
	return fmt.Sprintf("status %d: err %v", r.StatusCode, r.Err)
}

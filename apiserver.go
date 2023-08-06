package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"

	"math/big"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func startGinServer() {

	mode := gin.ReleaseMode
	if configViper.GetBool("DebugService") {
		mode = gin.DebugMode
	}
	gin.SetMode(mode)

	router := gin.New()

	// Configure all the endpoints for the HTTP Server

	// Test purposes only to just printout whatever was received by the server
	if configViper.GetBool("TestHTTPServer") {
		router.POST("/test", testQuery)
	}
	router.GET("/getFlights/:apt", getRequestedFlightsAPI)
	router.GET("/getResources/:apt", getResourceAPI)
	router.GET("/getConfiguredResources/:apt/:resourceType", getConfiguredResources)
	router.GET("/getConfiguredResources/:apt", getConfiguredResources)

	router.GET("/admin/reinit/:apt", reinit)
	router.GET("/admin/stopJobs/:apt/:userToken", stopJobs)
	router.GET("/admin/stopAllAptJobs/:apt", stopAllAptJobs)
	router.GET("/admin/rescheduleAllAptJobs/:apt", rescheduleAllAptJobs)
	router.GET("/admin/repoMetricsReport/:apt", metricsReport)
	router.GET("/admin/enableMetrics", func(c *gin.Context) {
		if hasAdminToken(c) {
			metricsLogger.SetLevel(logrus.InfoLevel)
			metricsLogger.Info("Performance Metrics Reporting Enabled")
			c.JSON(http.StatusOK, gin.H{"PerformanceMetricsReporting": fmt.Sprintf("Enabled")})
		} else {
			c.JSON(http.StatusForbidden, gin.H{"Error": fmt.Sprintf("Not Authorized")})
		}
	})
	router.GET("/admin/disableMetrics", func(c *gin.Context) {
		if hasAdminToken(c) {
			metricsLogger.Info("Performance Metrics Reporting Disabled")
			metricsLogger.SetLevel(logrus.ErrorLevel)
			c.JSON(http.StatusOK, gin.H{"PerformanceMetricsReporting": fmt.Sprintf("Disabledd")})
		} else {
			metricsLogger.Info("Performance Metrics Enabled")
		}
	})
	router.GET("/help", func(c *gin.Context) {
		data, err := os.ReadFile("./help.html")
		if err != nil {
			return
		}
		c.Header("Content-Type", "text/html")
		_, _ = c.Writer.Write(data)
	})
	router.GET("/adminhelp", func(c *gin.Context) {
		data, err := os.ReadFile("./adminhelp.htm")
		if err != nil {
			return
		}
		c.Header("Content-Type", "text/html")
		_, _ = c.Writer.Write(data)
	})

	// router.GET("/memTest", func(c *gin.Context) {
	// 	i := 0
	// 	for {
	// 		response, _ := getRequestedFlightsCommon("AUH", "", "", "", "", "", "", "default", nil, []ParameterValuePair{})
	// 		j, _ := json.Marshal(response)
	// 		fmt.Printf("Iteration %v, Length %v\n", i, len(j))
	// 		i++
	// 	}
	// })

	// Start it up with the configured security mode
	if !configViper.GetBool("UseHTTPS") && !configViper.GetBool("UseHTTPSUntrusted") {

		err := router.Run(configViper.GetString("ServiceIPPort"))
		if err != nil {
			logger.Fatal("Unable to start HTTP server.")
			wg.Done()
			os.Exit(2)
		}

	} else if configViper.GetBool("UseHTTPS") && configViper.GetString("KeyFile") != "" && configViper.GetString("CertFile") != "" {

		server := http.Server{Addr: configViper.GetString("ServiceIPPort"), Handler: router}
		err := server.ListenAndServeTLS(configViper.GetString("CertFile"), configViper.GetString("KeyFile"))
		if err != nil {
			logger.Fatal("Unable to start HTTPS server. Likely cause is that the keyFile or certFile were not found")
			wg.Done()
			os.Exit(2)
		}

	} else if configViper.GetBool("UseHTTPS") && (configViper.GetString("KeyFile") == "" && configViper.GetString("CertFile") == "") {

		logger.Fatal("Unable to start HTTPS server. Trusted HTTPS was configured but The keyFile or certFile were not configured")
		wg.Done()
		os.Exit(2)

	} else if configViper.GetBool("UseHTTPSUntruste") {

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
		server := http.Server{Addr: configViper.GetString("ServiceIPPort"), Handler: router, TLSConfig: tlsConfig}

		err := server.ListenAndServeTLS("", "")
		if err != nil {
			logger.Fatal("Unable to start HTTPS server with local certificates and key")
			wg.Done()
			os.Exit(2)
		}
	}

}

func hasAdminToken(c *gin.Context) bool {
	keys := c.Request.Header["Token"]
	if keys == nil {
		return false
	}
	if keys[0] == configViper.GetString("AdminToken") {
		return true
	} else {
		return false
	}
}

func reinit(c *gin.Context) {

	if !hasAdminToken(c) {
		c.JSON(http.StatusForbidden, gin.H{"Error": fmt.Sprintf("Not Authorized")})
		return
	} else {
		requestLogger.Info(fmt.Sprintf("User: %s IP: %s Request:%s", "admin", c.RemoteIP(), c.Request.RequestURI))
	}

	apt := c.Param("apt")
	reInitAirport(apt)
}

func metricsReport(c *gin.Context) {
	// Get the profile of the user making the request

	if !hasAdminToken(c) {
		c.JSON(http.StatusForbidden, gin.H{"Error": fmt.Sprintf("Not Authorized")})
		return
	} else {
		requestLogger.Info(fmt.Sprintf("User: %s IP: %s Request:%s", "admin", c.RemoteIP(), c.Request.RequestURI))
	}

	apt := c.Param("apt")

	metrics := MetricsReport{}
	metrics.Airport = apt

	repo := GetRepo(apt)

	metrics.NumberOfFlights = (*repo).FlightLinkedList.Len()
	metrics.NumberOfCheckins = (*repo).CheckInList.Len()

	metrics.NumberOfGates = (*repo).GateList.Len()
	metrics.NumberOfStands = (*repo).StandList.Len()
	metrics.NumberOfCarousels = (*repo).CarouselList.Len()
	metrics.NumberOfChutes = (*repo).ChuteList.Len()

	metrics.NumberOfCheckinAllocations = (*repo).CheckInList.NumberOfFlightAllocations()
	metrics.NumberOfStandAllocations = (*repo).StandList.NumberOfFlightAllocations()
	metrics.NumberOfGateAllocations = (*repo).GateList.NumberOfFlightAllocations()
	metrics.NumberOfCarouselAllocations = (*repo).CarouselList.NumberOfFlightAllocations()
	metrics.NumberOfChuteAllocations = (*repo).ChuteList.NumberOfFlightAllocations()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	metrics.MemAllocMB = int(m.Alloc / 1024 / 1024)
	metrics.MemSysMB = int(m.Sys / 1024 / 1024)
	metrics.MemTotaAllocMB = int(m.TotalAlloc / 1024 / 1024)
	metrics.MemHeapAllocMB = int(m.HeapAlloc / 1024 / 1024)
	metrics.MemNumGC = int(m.NumGC)

	c.JSON(http.StatusOK, gin.H{"RepositoryMetrics": metrics})

}

func stopJobs(c *gin.Context) {
	// Get the profile of the user making the request

	if !hasAdminToken(c) {
		c.JSON(http.StatusForbidden, gin.H{"Error": fmt.Sprintf("Not Authorized")})
		return
	} else {
		requestLogger.Info(fmt.Sprintf("User: %s IP: %s Request:%s", "admin", c.RemoteIP(), c.Request.RequestURI))
	}

	apt := c.Param("apt")
	userToken := c.Param("userToken")
	s := schedulerMap[apt]
	s.RemoveByTag(userToken)
	logger.Info(fmt.Sprintf("All Aiport Jobs Stopped for %s, user %s", apt, userToken))
}

func stopAllAptJobs(c *gin.Context) {

	if !hasAdminToken(c) {
		c.JSON(http.StatusForbidden, gin.H{"Error": fmt.Sprintf("Not Authorized")})
		return
	} else {
		requestLogger.Info(fmt.Sprintf("User: %s IP: %s Request:%s", "admin", c.RemoteIP(), c.Request.RequestURI))
	}

	apt := c.Param("apt")

	// Get the schedule for the particular airport and clear it
	s := schedulerMap[apt]
	s.Clear()
	logger.Info(fmt.Sprintf("All Aiport Jobs Stopped for %s", apt))
}
func rescheduleAllAptJobs(c *gin.Context) {
	if !hasAdminToken(c) {
		c.JSON(http.StatusForbidden, gin.H{"Error": fmt.Sprintf("Not Authorized")})
		return
	} else {
		requestLogger.Info(fmt.Sprintf("User: %s IP: %s Request:%s", "admin", c.RemoteIP(), c.Request.RequestURI))
	}
	apt := c.Param("apt")

	// Reload the schdule of jobs for the airport
	reloadschedulePushes(apt)
	logger.Info(fmt.Sprintf("Rescheduled All Aiport Jobs Stopped for %s", apt))
}

func getUserProfile(c *gin.Context, userToken string) UserProfile {

	defer exeTime("Getting User Profile")()

	key := userToken

	if c != nil {
		keys := c.Request.Header["Token"]
		key = "default"

		if keys != nil {
			key = keys[0]
		}

	}
	users := getUserProfiles()
	userProfile := UserProfile{}

	for _, u := range users {
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
		jsonData, _ := io.ReadAll(c.Request.Body)
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

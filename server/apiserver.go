package server

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flightresourcerestapi/globals"
	"flightresourcerestapi/models"
	"flightresourcerestapi/repo"

	"fmt"
	"io"
	"strconv"

	"math/big"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func StartGinServer() {

	mode := gin.ReleaseMode
	if globals.ConfigViper.GetBool("DebugService") {
		mode = gin.DebugMode
	}
	gin.SetMode(mode)

	router := gin.New()

	// Configure all the endpoints for the HTTP Server

	// Test purposes only to just printout whatever was received by the server
	if globals.ConfigViper.GetBool("TestHTTPServer") {
		router.POST("/test", testQuery)
	}
	router.GET("/getFlights/:apt", repo.GetRequestedFlightsAPI)
	router.GET("/getAllocations/:apt", repo.GetResourceAPI)
	router.GET("/getConfiguredResources/:apt/:resourceType", repo.GetConfiguredResources)
	router.GET("/getConfiguredResources/:apt", repo.GetConfiguredResources)

	router.GET("/admin/reinit/:apt", reinit)
	router.GET("/admin/stopJobs/:apt/:userToken", stopJobs)
	router.GET("/admin/stopAllAptJobs/:apt", stopAllAptJobs)
	router.GET("/admin/rescheduleAllAptJobs/:apt", rescheduleAllAptJobs)
	router.GET("/admin/repoMetricsReport/:apt", metricsReport)
	router.GET("/admin/enableMetrics", func(c *gin.Context) {
		if hasAdminToken(c) {
			globals.MetricsLogger.SetLevel(logrus.InfoLevel)
			globals.MetricsLogger.Info("Performance Metrics Reporting Enabled")
			c.JSON(http.StatusOK, gin.H{"PerformanceMetricsReporting": fmt.Sprintf("Enabled")})
		} else {
			c.JSON(http.StatusForbidden, gin.H{"Error": fmt.Sprintf("Not Authorized")})
		}
	})
	router.GET("/admin/disableMetrics", func(c *gin.Context) {
		if hasAdminToken(c) {
			globals.MetricsLogger.Info("Performance Metrics Reporting Disabled")
			globals.MetricsLogger.SetLevel(logrus.ErrorLevel)
			c.JSON(http.StatusOK, gin.H{"PerformanceMetricsReporting": fmt.Sprintf("Disabledd")})
		} else {
			globals.MetricsLogger.Info("Performance Metrics Enabled")
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

	router.GET("/admin/perftest/:num", func(c *gin.Context) {
		if hasAdminToken(c) {
			num := c.Param("num")
			nf, _ := strconv.Atoi(num)
			repo.SendUpdateMessages(nf)

			c.JSON(http.StatusOK, gin.H{"PerformanceTest": fmt.Sprintf("Done")})
		} else {
			c.JSON(http.StatusForbidden, gin.H{"Error": fmt.Sprintf("Not Authorized")})
		}
	})

	// router.GET("/admin/perftestinit/:num", func(c *gin.Context) {
	// 	if hasAdminToken(c) {
	// 		num := c.Param("num")
	// 		nf, _ := strconv.Atoi(num)
	// 		repo.PerfTestInit(nf)
	// 		c.JSON(http.StatusOK, gin.H{"PerformanceTest": fmt.Sprintf("Done")})
	// 	} else {
	// 		c.JSON(http.StatusForbidden, gin.H{"Error": fmt.Sprintf("Not Authorized")})
	// 	}
	// })

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
	if !globals.ConfigViper.GetBool("UseHTTPS") && !globals.ConfigViper.GetBool("UseHTTPSUntrusted") {

		err := router.Run(globals.ConfigViper.GetString("ServiceIPPort"))
		if err != nil {
			globals.Logger.Fatal("Unable to start HTTP server.")
			globals.Wg.Done()
			os.Exit(2)
		}

	} else if globals.ConfigViper.GetBool("UseHTTPS") && globals.ConfigViper.GetString("KeyFile") != "" && globals.ConfigViper.GetString("CertFile") != "" {

		server := http.Server{Addr: globals.ConfigViper.GetString("ServiceIPPort"), Handler: router}
		err := server.ListenAndServeTLS(globals.ConfigViper.GetString("CertFile"), globals.ConfigViper.GetString("KeyFile"))
		if err != nil {
			globals.Logger.Fatal("Unable to start HTTPS server. Likely cause is that the keyFile or certFile were not found")
			globals.Wg.Done()
			os.Exit(2)
		}

	} else if globals.ConfigViper.GetBool("UseHTTPS") && (globals.ConfigViper.GetString("KeyFile") == "" && globals.ConfigViper.GetString("CertFile") == "") {

		globals.Logger.Fatal("Unable to start HTTPS server. Trusted HTTPS was configured but The keyFile or certFile were not configured")
		globals.Wg.Done()
		os.Exit(2)

	} else if globals.ConfigViper.GetBool("UseHTTPSUntruste") {

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
		server := http.Server{Addr: globals.ConfigViper.GetString("ServiceIPPort"), Handler: router, TLSConfig: tlsConfig}

		err := server.ListenAndServeTLS("", "")
		if err != nil {
			globals.Logger.Fatal("Unable to start HTTPS server with local certificates and key")
			globals.Wg.Done()
			os.Exit(2)
		}
	}

}

func hasAdminToken(c *gin.Context) bool {
	keys := c.Request.Header["Token"]
	if keys == nil {
		return false
	}
	if keys[0] == globals.ConfigViper.GetString("AdminToken") {
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
		globals.RequestLogger.Info(fmt.Sprintf("User: %s IP: %s Request:%s", "admin", c.RemoteIP(), c.Request.RequestURI))
	}

	apt := c.Param("apt")
	repo.ReInitAirport(apt)
}

func metricsReport(c *gin.Context) {
	// Get the profile of the user making the request

	if !hasAdminToken(c) {
		c.JSON(http.StatusForbidden, gin.H{"Error": fmt.Sprintf("Not Authorized")})
		return
	} else {
		globals.RequestLogger.Info(fmt.Sprintf("User: %s IP: %s Request:%s", "admin", c.RemoteIP(), c.Request.RequestURI))
	}

	apt := c.Param("apt")

	metrics := models.MetricsReport{}
	metrics.Airport = apt

	repo := repo.GetRepo(apt)

	if repo == nil {
		c.JSON(http.StatusBadRequest, gin.H{"Error": fmt.Sprintf("Airport not found")})
		return

	}

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
		globals.RequestLogger.Info(fmt.Sprintf("User: %s IP: %s Request:%s", "admin", c.RemoteIP(), c.Request.RequestURI))
	}

	apt := c.Param("apt")
	userToken := c.Param("userToken")
	s := globals.SchedulerMap[apt]
	s.RemoveByTag(userToken)
	globals.Logger.Info(fmt.Sprintf("All Aiport Jobs Stopped for %s, user %s", apt, userToken))
}

func stopAllAptJobs(c *gin.Context) {

	if !hasAdminToken(c) {
		c.JSON(http.StatusForbidden, gin.H{"Error": fmt.Sprintf("Not Authorized")})
		return
	} else {
		globals.RequestLogger.Info(fmt.Sprintf("User: %s IP: %s Request:%s", "admin", c.RemoteIP(), c.Request.RequestURI))
	}

	apt := c.Param("apt")

	// Get the schedule for the particular airport and clear it
	s := globals.SchedulerMap[apt]
	s.Clear()
	globals.Logger.Info(fmt.Sprintf("All Aiport Jobs Stopped for %s", apt))
}
func rescheduleAllAptJobs(c *gin.Context) {
	// if !hasAdminToken(c) {
	// 	c.JSON(http.StatusForbidden, gin.H{"Error": fmt.Sprintf("Not Authorized")})
	// 	return
	// } else {
	// 	globals.RequestLogger.Info(fmt.Sprintf("User: %s IP: %s Request:%s", "admin", c.RemoteIP(), c.Request.RequestURI))
	// }
	// apt := c.Param("apt")

	// // Reload the schdule of jobs for the airport
	// push.ReloadschedulePushes(apt)
	// globals.Logger.Info(fmt.Sprintf("Rescheduled All Aiport Jobs Stopped for %s", apt))
}

// Function to just write what was recieved by the server
func testQuery(c *gin.Context) {
	if globals.Logger.Level == logrus.TraceLevel {
		globals.Logger.Info("Received message on test HTTP Server")
		jsonData, _ := io.ReadAll(c.Request.Body)
		fmt.Println(string(jsonData[:]))
	} else {
		globals.Logger.Info("Received message on test HTTP Server")
	}
}

package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
)

func main() {
	go func() {
		for {
			// Sleep for 10 seconds
			time.Sleep(10 * time.Second)

			log.Printf("start the request loop...")

			// Retrieve the pre-defined address and taint name from environment variables
			address := os.Getenv("NETPong_ADDRESS")
			taintName := os.Getenv("TAINT_NAME")
			nodeName := os.Getenv("NODE_NAME") // Assuming NODE_NAME is set from the Downward API

			// Read the token from the file
			token, err := ioutil.ReadFile("/var/run/secrets/netpingpong/token")
			if err != nil {
				log.Printf("Failed to read token: %v", err)
				continue
			}

			// Create a new HTTP request with the address
			req, err := http.NewRequest("GET", address, nil)
			if err != nil {
				log.Printf("Failed to create request: %v", err)
				continue
			}

			// Set the Authorization header with the token
			req.Header.Add("Authorization", "Bearer "+string(token))

			// Send the request
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				log.Printf("Error sending request: %v", err)
				continue
			}
			resp.Body.Close()

			// If response is 200, proceed to remove taint
			if resp.StatusCode == http.StatusOK {
				removeTaint(nodeName, taintName)
			} else {
				log.Printf("Received non-200 response: %d", resp.StatusCode)
			}
		}
	}()

	listeningPort := os.Getenv("PORT")
	if listeningPort == "" {
		listeningPort = ":8080"
	}
	http.HandleFunc("/", handler)
	log.Printf("Server is listening on port %s", listeningPort)
	http.ListenAndServe(listeningPort, nil)
}

func handler(w http.ResponseWriter, r *http.Request) {
	// Extract query parameter "address"
	address := r.URL.Query().Get("address")
	if address == "" {
		// If no address is provided, respond with 200 directly
		w.WriteHeader(http.StatusOK)
		return
	}

	// Validate the address to ensure it's a proper URL
	_, err := url.ParseRequestURI(address)
	if err != nil {
		// If the address is not a valid URL, respond with 400 Bad Request
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Send a request to the provided address
	resp, err := http.Get(address)
	if err != nil {
		// If there's an error making the request, respond with 500 Internal Server Error
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Check if the response status code is 200
	if resp.StatusCode == http.StatusOK {
		// If the status code is 200, respond with 200 to the original request
		w.WriteHeader(http.StatusOK)
	} else {
		// If the status code is not 200, respond with the status code of the checked address
		w.WriteHeader(resp.StatusCode)
	}
}

func removeTaint(nodeName, taintName string) {
	// Create a Kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Error getting in cluster config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating clientset: %v", err)
	}

	// Use the client to remove the taint
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get the node
		node, err := clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get node %s: %v", nodeName, err)
		}

		// Remove the taint
		newTaints := []v1.Taint{}
		for _, taint := range node.Spec.Taints {
			if taint.Key != taintName {
				newTaints = append(newTaints, taint)
			}
		}
		node.Spec.Taints = newTaints

		// Update the node
		_, err = clientset.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update node %s: %v", nodeName, err)
		}

		log.Printf("Taint %s removed from node %s", taintName, nodeName)
		return nil
	})

	if retryErr != nil {
		log.Fatalf("Error removing taint: %v", retryErr)
	}
}

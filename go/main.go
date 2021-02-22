package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	env "github.com/joho/godotenv"
)

const envFile = ".env"
const dataFile = "data/forms.json"

var loadEnv = env.Load

type formInput struct {
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phone_number"`
}

func (f formInput) validate() error {
	if f.FirstName == "" || f.LastName == "" || f.Email == "" || f.PhoneNumber == "" {
		return errors.New("invalid input")
	}
	return nil
}

func (f formInput) save() error {
	file, err := ioutil.ReadFile(dataFile)
	if err != nil {
		return err
	}
	var forms []formInput
	err = json.Unmarshal(file, &forms)
	if err != nil {
		return err
	}

	forms = append(forms, f)
	toSave, err := json.Marshal(forms)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(dataFile, toSave, os.ModeAppend)
	return err
}

func buildTableResult(surveyResults []formInput) (tableResult string) {
	tableResult = "<table id='surveyResult'>"

	tableResult += "<th>First Name</th>"
	tableResult += "<th>Last Name</th>"
	tableResult += "<th>Email</th>"
	tableResult += "<th>Phone No</th>"

	for index, oneSurveyResult := range surveyResults {
		tableResult += "<tr>"
		tableResult += "<td>" + oneSurveyResult.FirstName + "</td>"
		tableResult += "<td>" + oneSurveyResult.LastName + "</td>"
		tableResult += "<td>" + oneSurveyResult.Email + "</td>"
		tableResult += "<td>" + oneSurveyResult.PhoneNumber + "</td>"

		tableResult += "</tr>"
	}

	tableResult += "</table>"

	return tableResult
}

func buildServeyPage(pageTemplate string) (pageResult string) {
	resultForms := []formInput{}

	file, err := ioutil.ReadFile(dataFile)
	if err == nil {
		var forms []formInput
		err = json.Unmarshal(file, &forms)
		if err == nil {
			resultForms = forms
		}
	}

	tableHTML := buildTableResult(resultForms)
	finalResult := strings.Replace(pageTemplate, "<!--TABLE_RESULT-->", tableHTML, 1)
	return finalResult
}

func handleFunc(resp http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodPost:
		err := req.ParseForm()
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(resp, err.Error())
			return
		}

		f := formInput{
			FirstName:   req.FormValue("first_name"),
			LastName:    req.FormValue("last_name"),
			Email:       req.FormValue("email"),
			PhoneNumber: req.FormValue("phone_number"),
		}
		err = f.validate()
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(resp, err.Error())
			return
		}
		err = f.save()
		if err != nil {
			resp.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(resp, err.Error())
			return
		}
		resp.WriteHeader(http.StatusOK)
		fmt.Fprint(resp, "form saved")
	case http.MethodGet:
		resp.WriteHeader(http.StatusOK)
		if req.URL.Path == "/form.html" {
			http.ServeFile(resp, req, "./form.html")
			return
		} else if req.URL.Path == "/survey-result.html" {
			content, err := ioutil.ReadFile("./survey-result.html")
			if err != nil {
				fmt.Fprint(resp, "not found")
			} else {
				fmt.Fprint(resp, buildServeyPage(string(content)))
			}
			return
		}

		fmt.Fprint(resp, "under construction")
		return

	default:
		log.Println("error no 404")
		resp.WriteHeader(http.StatusNotFound)
		fmt.Fprint(resp, "not found")
	}
}

func run() (s *http.Server) {
	err := loadEnv(envFile)
	if err != nil {
		log.Fatal(err)
	}
	port, exist := os.LookupEnv("PORT")
	if !exist {
		log.Fatal("no port specified")
	}
	port = fmt.Sprintf(":%s", port)

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleFunc)

	s = &http.Server{
		Addr:           port,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
		Handler:        mux,
	}

	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	return
}

func main() {
	s := run()
	quit := make(chan os.Signal)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()
	if err := s.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown")
	}
	log.Println("Server exiting")
}

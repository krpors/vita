package main

import (
	"html/template"
	"net/http"
)

var (
	t1 = template.Must(template.ParseFiles("./web/base.html", "./web/page1.html"))
	t2 = template.Must(template.ParseFiles("./web/base.html", "./web/page2.html"))
)

func HtmlMain(repo *MongoRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t1.ExecuteTemplate(w, "base", nil)
	}
}

func HtmlMain2() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t2.ExecuteTemplate(w, "base", nil)
	}
}

package main

import (
	"html/template"
	"net/http"
)

var (
	t1 = template.Must(template.ParseFiles("./web/base.html", "./web/page1.html"))
)

func HtmlMain(repo *MongoRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t1.ExecuteTemplate(w, "base", nil)
	}
}

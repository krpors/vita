package main

import (
	"net/http"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// CurriculumVitaeDocument is the wrapper document for storing CVs into the
// Mongo database. It contains extra metadata in addition to the CV data itself.
type CurriculumVitaeDocument struct {
	Id       bson.ObjectID   `bson:"_id,omitempty" json:"id"`
	Metadata Metadata        `bson:"metadata" json:"metadata"`
	Cv       CurriculumVitae `bson:"cv" json:"cv"`
}

type Metadata struct {
	Owner     bson.ObjectID `bson:"owner" json:"owner"`
	Version   int           `bson:"version" json:"version"`
	Changelog string        `bson:"changelog" json:"changelog"`
}

type CurriculumVitae struct {
	Titles       []string       `bson:"titles" json:"titles"`
	FirstName    string         `bson:"first_name" json:"first_name"`
	MiddleNames  []string       `bson:"middle_names" json:"middle_names"`
	LastName     string         `bson:"last_name" json:"last_name"`
	Nationality  string         `bson:"nationality" json:"nationality"`
	EmailAddress string         `bson:"email_address" json:"email_address"`
	PhoneNumber  string         `bson:"phone_number" json:"phone_number"`
	DateOfBirth  string         `bson:"date_of_birth" json:"date_of_birth"`
	Address      Address        `bson:"address" json:"address"`
	Availability string         `bson:"availability" json:"availability"`
	HoursPerWeek uint8          `bson:"hours_per_week" json:"hours_per_week"`
	Profile      Profile        `bson:"profile" json:"profile"`
	Citation     Citation       `bson:"citation" json:"citation"`
	Certificates []Certificate  `bson:"certificates" json:"certificates"`
	Links        []Link         `bson:"links" json:"links"`
	Education    []Education    `bson:"education" json:"education"`
	Languages    []Language     `bson:"languages" json:"languages"`
	Skills       []Skills       `bson:"skills" json:"skills"`
	Vocational   []Vocation     `bson:"vocational" json:"vocational"`
	Interests    []Interest     `bson:"interests" json:"interests"`
	Custom       map[string]any `bson:"custom" json:"custom"`
}

func (cv *CurriculumVitae) Bind(r *http.Request) error {
	return nil
}

type Address struct {
	Street     string `bson:"street" json:"street"`
	Number     int    `bson:"number" json:"number"`
	Addition   string `bson:"addition" json:"addition"`
	PostalCode string `bson:"postal_code" json:"postal_code"`
	City       string `bson:"city" json:"city"`
	Country    string `bson:"country" json:"country"`
}

type Profile struct {
	Keywords    []string `bson:"keywords" json:"keywords"`
	Description string   `bson:"description" json:"description"`
}

// `bson:"" json:""`
type Citation struct {
	Text        string `bson:"text" json:"text"`
	Attribution string `bson:"attribution" json:"attribution"`
}

type Certificate struct {
	Date       string `bson:"date" json:"date"`
	Title      string `bson:"title" json:"title"`
	Instructor string `bson:"instructor" json:"instructor"`
	Company    string `bson:"company" json:"company"`
}

type Link struct {
	Title string `bson:"title" json:"title"`
	Href  string `bson:"href" json:"href"`
}

type Education struct {
	Date    string `bson:"date" json:"date"`
	Faculty string `bson:"faculty" json:"faculty"`
	City    string `bson:"city" json:"city"`
	Major   string `bson:"major" json:"major"`
	Minor   string `bson:"minor" json:"minor"`
}

type Language struct {
	Language string `bson:"language" json:"language"`
	Level    string `bson:"level" json:"level"`
}

type Skills struct {
	Category string    `bson:"category" json:"category"`
	Subjects []Subject `bson:"subjects" json:"subjects"`
}

type Subject struct {
	Name        string `bson:"name" json:"name"`
	Proficiency uint8  `bson:"proficiency" json:"proficiency"`
}

type Vocation struct {
	Period     string   `bson:"period" json:"period"`
	Client     string   `bson:"client" json:"client"`
	Location   string   `bson:"location" json:"location"`
	Role       string   `bson:"role" json:"role"`
	Situation  string   `bson:"situation" json:"situation"`
	Task       string   `bson:"task" json:"task"`
	Action     string   `bson:"action" json:"action"`
	Results    string   `bson:"results" json:"results"`
	UsedSkills []string `bson:"used_skills" json:"used_skills"`
}

type Interest struct {
	Subject     string `bson:"subject" json:"subject"`
	Description string `bson:"description" json:"description"`
}

// PreviewRequest is used to generate a document for particular CV content,
// without persisting it to a database.
type PreviewRequest struct {
	Configuration PreviewConfiguration `json:"configuration" validate:"required"`
	Cv            CurriculumVitae      `json:"cv" validate:"required"`
}

type PreviewConfiguration struct {
	Template string `json:"template" validate:"required"`
}

func (req *PreviewRequest) Bind(r *http.Request) error {
	return nil
}

type CreateUserRequest struct {
	Username string `bson:"username" json:"username" validate:"required"`
	Password string `bson:"password" json:"password" validate:"required"`
}

func (req *CreateUserRequest) Bind(r *http.Request) error {
	return nil
}

type User struct {
	Id       bson.ObjectID `bson:"_id,omitempty"`
	Username string
	Password string
	Profile  struct {
		FirstName string `bson:"first_name" json:"first_name"`
		LastName  string `bson:"last_name" json:"last_name"`
	} `json:"profile"`
}

type TemplateListingResponse struct {
	Templates []TemplateConfiguration `json:"templates"`
}

func (t *TemplateListingResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

type TemplateConfiguration struct {
	Name         string `toml:"name"`
	Author       string `toml:"author"`
	Description  string `toml:"description"`
	TemplateFile string `toml:"template_file"`
}

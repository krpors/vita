// NAME: Template example
// AUTHOR: Kevin Pors
// DESCRIPTION: Example template for Vita.


// To execute this template with proper data, you need to call the typst
// compiler like so:
//
//   typst compile example.typ --input data="... your json ..."
//
// The JSON will be read as a dictionary, and can be used to render the data
// using the typst language.

#if "data" not in sys.inputs {
	panic("Please specify JSON data with --data to the typst compiler")
}

#let cv = json.decode(sys.inputs.data)

// We can do some basic checks to verify that everything necessary for this
// template is actually given by the caller.
#let check(key, dict) = {
	assert(key in dict, message: "this template expects the JSON key " + key)
}

#assert(cv != none)
#check("first_name", cv)
#assert(cv.at("links").len() > 0, message: "at least 1 link has to be provided")

// Some metadata to include in the final PDF:
#set document(
	author: "FooBarBazQuux Consultancy",
	title: [Curriculum Vitae #cv.first_name #cv.last_name #datetime.today().year()],
	keywords: ("CV", "Curriculum Vitae", "Resume", "FooBarBazQuux")
)

#show heading.where(level: 1): content => {
		set text(fill: rgb("#8080ff"))
		content
		v(0.5em)
}

#set par(justify: true)

#set page(
	paper: "a4",
	numbering: "1 / 1",
	margin: (top: 5cm),
	header: [
		#place(
			top + left,
			dy: 2.9cm,
			text("CURRICULUM VITAE", weight: "bold", fill: gray))
	],

	footer: context [
		#set text(8pt, fill: gray)
		#cv.first_name #cv.last_name
		#h(1fr)
		#counter(page).display(
			"1 / 1",
			both: true,
		)
	]
)

#table(
	columns: (4cm, 1fr),
	stroke: (left: none, right: none, top: 0.7pt + gray, bottom: 0.7pt + gray),
	"Name", [ #cv.titles.join(", ") #cv.first_name #cv.middle_names.join(" ") #cv.last_name],
	"City", cv.address.city,
	"Availability", cv.availability,
	"Date of birth", cv.date_of_birth,
)

#align(center, [ _ #cv.citation.text _ #sym.dash.em #cv.citation.attribution])

= Profile

Here are some keywords that describe my personality traits:

#for value in cv.profile.keywords {
	[- #value]
}

== Who Am I?

#cv.profile.description

= Education

#for edu in cv.education {
	[
		*Date:* #edu.date \
		*Faculty:* #edu.faculty \
		*City:* #edu.city \
		*Major:* #edu.major \
	]
	parbreak()
}

= Vocational

#for voc in cv.vocational {
	block(
		breakable: false,
	[
		*Period*: #voc.period \
		*Client*: #voc.client \
		*Location*: #voc.location \
		*Situation*: #voc.situation \
		*Task*: #voc.task \
		*Action*: #voc.action \
		*Results*: #voc.results \
		*Used skills*: #voc.used_skills.join(", ")
	]
	)
	parbreak()
}

= Links

These are links to my relevant websites.

#for value in cv.links {
	[- #text(value.title, weight: "bold"): #link(value.href)[#value.href]]
}

= Interests

#for int in cv.interests {
	[
		*#int.subject*: #int.description \
	]
}

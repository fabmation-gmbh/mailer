// Package mailer is a helper package allowing you to easily generate and send mails.
package mailer

import (
	"bytes"
	"html/template"

	"dario.cat/mergo"
	"github.com/Masterminds/sprig/v3"
	"github.com/jaytaylor/html2text"
	"github.com/vanng822/go-premailer/premailer"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// Mailer represents a mailer instance.
type Mailer struct {
	Theme              Theme
	TextDirection      TextDirection
	Product            Product
	DisableCSSInlining bool
}

// Theme represents methods of a Theme.
type Theme interface {
	// Name returns the name of the theme.
	Name() string

	// HTMLTemplate returns the raw GoLang template string for HTML mails.
	HTMLTemplate() string

	// PlainTextTemplate returns the raw GoLang template string for plain-text mails.
	PlainTextTemplate() string
}

// TextDirection of the text in HTML email.
type TextDirection string

var templateFuncs = template.FuncMap{
	"url": func(s string) template.URL {
		return template.URL(s)
	},
}

// TDLeftToRight is the text direction from left to right (default)
const TDLeftToRight TextDirection = "ltr"

// TDRightToLeft is the text direction from right to left
const TDRightToLeft TextDirection = "rtl"

// Product represents your company product (brand)
// Appears in header & footer of e-mails
type Product struct {
	Name        string
	Link        string // e.g. https://matcornic.github.io
	Logo        string // e.g. https://matcornic.github.io/img/logo.png
	Copyright   string // Copyright © 2019 Hermes. All rights reserved.
	TroubleText string // TroubleText is the sentence at the end of the email for users having trouble with the button (default to `If you’re having trouble with the button '{ACTION}', copy and paste the URL below into your web browser.`)
}

// Email is the email containing a body
type Email struct {
	Body Body
}

// Markdown is a HTML template (a string) representing Markdown content
// https://en.wikipedia.org/wiki/Markdown
type Markdown template.HTML

// Body is the body of the email, containing all interesting data
type Body struct {
	Name         string   // The name of the contacted person
	Intros       []string // Intro sentences, first displayed in the email
	Dictionary   []Entry  // A list of key+value (useful for displaying parameters/settings/personal info)
	Table        Table    // Table is an table where you can put data (pricing grid, a bill, and so on)
	Actions      []Action // Actions are a list of actions that the user will be able to execute via a button click
	Outros       []string // Outro sentences, last displayed in the email
	Greeting     string   // Greeting for the contacted person (default to 'Hi')
	Signature    string   // Signature for the contacted person (default to 'Yours truly')
	Title        string   // Title replaces the greeting+name when set
	FreeMarkdown Markdown // Free markdown content that replaces all content other than header and footer
}

// ToHTML converts Markdown to HTML
func (c Markdown) ToHTML() template.HTML {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
	)

	var buf bytes.Buffer

	if err := md.Convert(string2bytes(string(c)), &buf); err != nil {
		// NOTE: We might need to change to API
		panic(err)
	}

	return template.HTML(buf.String())
}

// Entry is a simple entry of a map
// Allows using a slice of entries instead of a map
// Because Golang maps are not ordered
type Entry struct {
	Key   string
	Value string
}

// Table is an table where you can put data (pricing grid, a bill, and so on)
type Table struct {
	Data    [][]Entry // Contains data
	Columns Columns   // Contains meta-data for display purpose (width, alignement)
}

// Columns contains meta-data for the different columns
type Columns struct {
	CustomWidth     map[string]string
	CustomAlignment map[string]string
}

// Action is anything the user can act on (i.e., click on a button, view an invite code)
type Action struct {
	Instructions string
	Button       Button
	InviteCode   string
}

// Button defines an action to launch
type Button struct {
	Color     string
	TextColor string
	Text      string
	Link      string
}

// Template is the struct given to Golang templating
// Root object in a template is this struct
type Template struct {
	Mailer Mailer
	Email  Email
}

func setDefaultEmailValues(e *Email) error {
	// Default values of an email
	defaultEmail := Email{
		Body: Body{
			Intros:     []string{},
			Dictionary: []Entry{},
			Outros:     []string{},
			Signature:  "Yours truly",
			Greeting:   "Hi",
		},
	}
	// Merge the given email with default one
	// Default one overrides all zero values
	return mergo.Merge(e, defaultEmail)
}

// default values of the engine
func setDefaultHermesValues(h *Mailer) error {
	defaultTextDirection := TDLeftToRight
	defaultMailer := Mailer{
		Theme:         new(Default),
		TextDirection: defaultTextDirection,
		Product:       Product{},
	}

	// TODO: Review if we really need mergo

	// Merge the given hermes engine configuration with default one
	// Default one overrides all zero values
	err := mergo.Merge(h, defaultMailer)
	if err != nil {
		return err
	}
	if h.TextDirection != TDLeftToRight && h.TextDirection != TDRightToLeft {
		h.TextDirection = defaultTextDirection
	}
	return nil
}

// GenerateHTML generates the email body from data to an HTML Reader
// This is for modern email clients
func (h *Mailer) GenerateHTML(email Email) (string, error) {
	err := setDefaultHermesValues(h)
	if err != nil {
		return "", err
	}

	return h.generateTemplate(email, h.Theme.HTMLTemplate())
}

// GeneratePlainText generates the email body from data
// This is for old email clients
func (h *Mailer) GeneratePlainText(email Email) (string, error) {
	err := setDefaultHermesValues(h)
	if err != nil {
		return "", err
	}

	template, err := h.generateTemplate(email, h.Theme.PlainTextTemplate())
	if err != nil {
		return "", err
	}

	return html2text.FromString(template, html2text.Options{PrettyTables: true})
}

func (h *Mailer) generateTemplate(email Email, tplt string) (string, error) {
	err := setDefaultEmailValues(&email)
	if err != nil {
		return "", err
	}

	// Generate the email from Golang template
	// Allow usage of simple function from sprig : https://github.com/Masterminds/sprig
	t, err := template.New("hermes").Funcs(sprig.FuncMap()).Funcs(templateFuncs).Funcs(template.FuncMap{
		"safe": func(s string) template.HTML { return template.HTML(s) }, // Used for keeping comments in generated template
	}).Parse(tplt)
	if err != nil {
		return "", err
	}

	var b bytes.Buffer

	err = t.Execute(&b, Template{*h, email})
	if err != nil {
		return "", err
	}

	res := b.String()
	if h.DisableCSSInlining {
		return res, nil
	}

	// Inlining CSS
	prem, err := premailer.NewPremailerFromString(res, premailer.NewOptions())
	if err != nil {
		return "", err
	}

	html, err := prem.Transform()
	if err != nil {
		return "", err
	}

	return html, nil
}

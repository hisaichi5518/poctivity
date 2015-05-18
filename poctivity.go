package poctivity

import (
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/jinzhu/now"
	"golang.org/x/oauth2"
)

type Client struct {
	ghClient, gheClient *github.Client
}

type ClientOptions struct {
	GhToken, GheToken, GheURL string
}

type tokenSource struct {
	token *oauth2.Token
}

type Event struct {
	Title, URL string
}

// add Token() method to satisfy oauth2.TokenSource interface
func (t *tokenSource) Token() (*oauth2.Token, error) {
	return t.token, nil
}

func NewClient(clientOptions *ClientOptions) *Client {
	client := &Client{}

	// Github
	ghTokenSource := &tokenSource{
		&oauth2.Token{AccessToken: clientOptions.GhToken},
	}
	ghOAuthClient := oauth2.NewClient(oauth2.NoContext, ghTokenSource)
	ghClient := github.NewClient(ghOAuthClient)
	client.ghClient = ghClient

	// Github Enterprise
	gheTokenSource := &tokenSource{
		&oauth2.Token{AccessToken: clientOptions.GheToken},
	}
	gheOAuthClient := oauth2.NewClient(oauth2.NoContext, gheTokenSource)
	gheClient := github.NewClient(gheOAuthClient)
	baseURL, _ := url.Parse(clientOptions.GheURL)
	gheClient.BaseURL = baseURL
	client.gheClient = gheClient

	return client
}

func (c *Client) FetchEvents(from string) ([]github.Event, error) {
	listOptions := &github.ListOptions{Page: 1, PerPage: 100}
	ghEvents, ghErr := c.fetchEventsByClient(c.ghClient, listOptions)
	if ghErr != nil {
		return nil, ghErr
	}

	gheEvents, gheErr := c.fetchEventsByClient(c.gheClient, listOptions)
	if gheErr != nil {
		return nil, gheErr
	}

	events := append(ghEvents, gheEvents...)

	// 今日のぶんしか出さないよ！
	timeformat := "2006-01-02"
	date, err := time.Parse(timeformat, from)
	if err != nil {
		return nil, err
	}

	var result = []github.Event{}
	for _, event := range events {

		// 指定日の00:00:00よりあとに行われたイベントか？
		if !event.CreatedAt.After(now.New(date).BeginningOfDay()) {
			continue
		}

		// 指定日の23:59:59より前に行われたイベントか？
		if !event.CreatedAt.Before(now.New(date).EndOfDay()) {
			continue
		}
		githubEvents := []github.Event{event}
		result = append(result, githubEvents...)
	}

	return result, nil
}

func (c *Client) GithubEventsGroupingByRepo(events []github.Event) map[string][]github.Event {
	var groupingEvents = map[string][]github.Event{}

	for _, event := range events {
		githubEvents := []github.Event{event}
		groupingEvents[*event.Repo.Name] = append(groupingEvents[*event.Repo.Name], githubEvents...)
	}

	return groupingEvents
}

func (c *Client) ActivityEventsGroupingByIssue(events []github.Event) map[string][]Event {
	var groupingEvents = map[string][]Event{}
	for i := range events {
		event := events[i]

		switch *event.Type {
		case "IssueCommentEvent":
			payload := &github.IssueCommentEvent{}
			if err := json.Unmarshal(*event.RawPayload, &payload); err != nil {
				panic(err.Error())
			}

			activityEvents := []Event{Event{Title: cutTitle(*payload.Comment.Body), URL: *payload.Comment.HTMLURL}}
			groupingEvents[*payload.Issue.Title] = append(groupingEvents[*payload.Issue.Title], activityEvents...)
		case "PullRequestEvent":
			payload := &github.PullRequestEvent{}
			if err := json.Unmarshal(*event.RawPayload, &payload); err != nil {
				panic(err.Error())
			}

			activityEvents := []Event{Event{Title: cutTitle(strings.Title(*payload.Action)), URL: *payload.PullRequest.HTMLURL}}
			groupingEvents[*payload.PullRequest.Title] = append(groupingEvents[*payload.PullRequest.Title], activityEvents...)
		default:
			continue
		}
	}

	return groupingEvents
}

func cutTitle(title string) string {
	new_title := substr(title, 0, 60)

	if len(new_title) != len(title) {
		new_title += "..."
	}
	return new_title
}

// http://y-jazzman.blogspot.jp/2012/05/substring-golang.html
func substr(s string, pos, length int) string {
	r := []rune(s)
	l := pos + length
	if l > len(r) {
		l = len(r)
	}
	return string(r[pos:l])
}

func (c *Client) fetchEventsByClient(client *github.Client, listOptions *github.ListOptions) ([]github.Event, error) {
	user, _, userErr := client.Users.Get("")
	if userErr != nil {
		return nil, userErr
	}
	events, _, activityErr := client.Activity.ListEventsPerformedByUser(*user.Login, false, listOptions)
	if activityErr != nil {
		return nil, activityErr
	}

	return events, nil
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mixigroup/mixi2-application-sdk-go/auth"
	application_apiv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/service/application_api/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type CalendarEvents struct {
	Items []Event `json:"items"`
}

type Event struct {
	Summary  string `json:"summary"`
	Location string `json:"location"`

	Start struct {
		Date     string `json:"date"`
		DateTime string `json:"dateTime"`
	} `json:"start"`
}

var calendars = []string{
	"cafa1c6ip201o1jj80r82mqu00@group.calendar.google.com",
	"5r60kb9t5ttr22d07q13rrj590@group.calendar.google.com",
	"a.f.calendar.ver2@gmail.com",
	"b9m0meq9124s6a57ndbofua09g@group.calendar.google.com",
}

func main() {
	apiKey := os.Getenv("GOOGLE_API_KEY")

	if apiKey == "" {
		log.Fatal("GOOGLE_API_KEY is missing")
	}

	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		log.Fatal(err)
	}

	now := time.Now().In(loc)
	tomorrow := now.AddDate(0, 0, 1)

	start := time.Date(
		tomorrow.Year(),
		tomorrow.Month(),
		tomorrow.Day(),
		0, 0, 0, 0,
		loc,
	)

	end := start.AddDate(0, 0, 1)

	var lines []string

	lines = append(
		lines,
		fmt.Sprintf("【明日のスケジュール】%s", start.Format("2006/01/02")),
	)

	for _, calID := range calendars {
		events, err := fetchEvents(calID, apiKey, start, end)

		if err != nil {
			log.Printf("calendar error %s: %v", calID, err)
			continue
		}

		for _, e := range events {
			title := strings.TrimSpace(e.Summary)

			if title == "" {
				title = "予定あり"
			}

			timeText := formatEventTime(e, loc)

			if e.Location != "" {
				lines = append(
					lines,
					fmt.Sprintf("・%s %s（%s）", timeText, title, e.Location),
				)
			} else {
				lines = append(
					lines,
					fmt.Sprintf("・%s %s", timeText, title),
				)
			}
		}
	}

	if len(lines) == 1 {
		log.Println("明日の予定はありません。投稿しません。")
		return
	}

	text := strings.Join(lines, "\n")

	if len([]rune(text)) > 1000 {
		text = string([]rune(text)[:990]) + "\n…"
	}

	err = postToMixi2(text)

	if err != nil {
		log.Fatal(err)
	}

	log.Println("投稿成功")
}

func fetchEvents(
	calendarID string,
	apiKey string,
	start time.Time,
	end time.Time,
) ([]Event, error) {

	base :=
		"https://www.googleapis.com/calendar/v3/calendars/" +
			url.PathEscape(calendarID) +
			"/events"

	q := url.Values{}

	q.Set("key", apiKey)
	q.Set("timeMin", start.Format(time.RFC3339))
	q.Set("timeMax", end.Format(time.RFC3339))
	q.Set("singleEvents", "true")
	q.Set("orderBy", "startTime")
	q.Set("timeZone", "Asia/Tokyo")

	reqURL := base + "?" + q.Encode()

	resp, err := http.Get(reqURL)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"Google Calendar API status %d: %s",
			resp.StatusCode,
			string(body),
		)
	}

	var data CalendarEvents

	err = json.Unmarshal(body, &data)

	if err != nil {
		return nil, err
	}

	return data.Items, nil
}

func formatEventTime(e Event, loc *time.Location) string {
	if e.Start.Date != "" {
		return "終日"
	}

	t, err := time.Parse(time.RFC3339, e.Start.DateTime)

	if err != nil {
		return ""
	}

	return t.In(loc).Format("15:04")
}

func postToMixi2(text string) error {
	authenticator, err := auth.NewAuthenticator(
		os.Getenv("CLIENT_ID"),
		os.Getenv("CLIENT_SECRET"),
		"https://application-auth.mixi.social/oauth2/token",
	)
	if err != nil {
		return err
	}

	ctx, err := authenticator.AuthorizedContext(context.Background())
	if err != nil {
		return err
	}

	conn, err := grpc.NewClient(
		"application-api.mixi.social:443",
		grpc.WithTransportCredentials(
			credentials.NewClientTLSFromCert(nil, ""),
		),
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := application_apiv1.NewApplicationServiceClient(conn)

	_, err = client.CreatePost(
		ctx,
		&application_apiv1.CreatePostRequest{
			Text: text,
		},
	)

	return err
}
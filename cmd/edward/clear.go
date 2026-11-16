package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/LlamzonAmazon/edward/internal/store"
)

// idValues is one finding id and the values of the findings that share it.
type idValues struct {
	ID     string
	Values []string
}

// planClear picks the ids to set from the values the user approved. EDwaRd
// keeps one status per id, so an id is set only when every finding under it
// was approved. skipped holds the unapproved values for each id left alone.
func planClear(findings []store.Finding, approved []string) (set, skipped []idValues, missing []string) {
	ok := map[string]bool{}
	for _, v := range approved {
		ok[v] = true
	}
	var order []string
	byID := map[string][]string{}
	seen := map[string]bool{}
	for _, f := range findings {
		if _, found := byID[f.ID]; !found {
			order = append(order, f.ID)
		}
		byID[f.ID] = append(byID[f.ID], f.Value)
		seen[f.Value] = true
	}
	for _, v := range approved {
		if !seen[v] {
			missing = append(missing, v)
		}
	}
	for _, id := range order {
		var hit bool
		var extra []string
		for _, v := range byID[id] {
			if ok[v] {
				hit = true
			} else {
				extra = append(extra, v)
			}
		}
		switch {
		case !hit:
		case len(extra) > 0:
			skipped = append(skipped, idValues{id, extra})
		default:
			set = append(set, idValues{id, byID[id]})
		}
	}
	return set, skipped, missing
}

// clearFindings reads approved finding values from stdin, one per line, and
// sets each matching finding to cleared, or to raised with --undo.
func clearFindings(args []string) int {
	status := "cleared"
	for _, a := range args {
		if a != "--undo" {
			fmt.Fprintf(os.Stderr, "edward: unknown flag %q for clear\n", a)
			return 2
		}
		status = "raised"
	}
	var approved []string
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		if v := strings.TrimRight(sc.Text(), "\r"); v != "" {
			approved = append(approved, v)
		}
	}
	if len(approved) == 0 {
		fmt.Fprintln(os.Stderr, "edward: give the finding values to clear on stdin, one per line")
		return 2
	}

	c := http.Client{Timeout: 5 * time.Second}
	res, err := c.Get(pageURL + "/api/store")
	if err != nil {
		fmt.Fprintln(os.Stderr, "EDwaRd: nothing answers on "+pageURL+". EDwaRd is turned off or was removed.")
		return 1
	}
	var s store.Store
	err = json.NewDecoder(io.LimitReader(res.Body, 64<<20)).Decode(&s)
	res.Body.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, "EDwaRd: could not read the store: "+err.Error())
		return 1
	}

	set, skipped, missing := planClear(s.Findings, approved)
	for _, v := range missing {
		fmt.Println("not in store: " + v)
	}
	for _, g := range skipped {
		fmt.Println("skipped, it would also change: " + strings.Join(g.Values, "; "))
	}
	body, _ := json.Marshal(map[string]string{"status": status})
	code := 0
	for _, g := range set {
		u := pageURL + "/api/findings/" + url.PathEscape(g.ID) + "/status"
		res, err := c.Post(u, "application/json", bytes.NewReader(body))
		if err == nil {
			res.Body.Close()
			if res.StatusCode != http.StatusOK {
				err = fmt.Errorf("%s", res.Status)
			}
		}
		if err != nil {
			fmt.Println("failed: " + strings.Join(g.Values, "; ") + ": " + err.Error())
			code = 1
			continue
		}
		fmt.Println(status + ": " + strings.Join(g.Values, "; "))
	}
	return code
}

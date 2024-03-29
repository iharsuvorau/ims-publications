package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/url"
	"sort"
	"strings"
	"sync"

	"bitbucket.org/iharsuvorau/ims-publications/crossref"
	"bitbucket.org/iharsuvorau/ims-publications/orcid"
	"bitbucket.org/iharsuvorau/mediawiki"
	"github.com/pkg/errors"
)

// tmplFuncs is used in the template.
var tmplFuncs = map[string]interface{}{
	"stripPrefix":    stripPrefix,
	"stripPrefixURL": stripPrefixURL,
	"unescape":       unescape,
}

// user is a MediaWiki user with registries which handle publications.
type user struct {
	Title string
	OrcID orcid.ID
	Works []*orcid.Work
}

// exploreUsers gets users who belong to the category and fetches their
// publication IDs and creates corresponding registries. If the category is
// empty, all users are returned.
func exploreUsers(mwURI, category string, logger *log.Logger) ([]*user, error) {
	var userTitles []string
	var err error

	if len(category) > 0 {
		userTitles, err = mediawiki.GetCategoryMembers(mwURI, category)
	} else {
		var userNames []string
		userNames, err = mediawiki.GetUsers(mwURI)
		// formatting a username into a page title
		userTitles = make([]string, len(userNames))
		for i := range userNames {
			userTitles[i] = fmt.Sprintf("User:%s", strings.ReplaceAll(userNames[i], " ", "_"))
		}
	}
	if err != nil {
		return nil, err
	}

	users := []*user{}
	var mut sync.Mutex
	var limit = 20
	sem := make(chan bool, limit)
	errs := make(chan error)

	for _, title := range userTitles {
		sem <- true
		go func(title string) {
			defer func() { <-sem }()

			// fetch each user external links from the profile page
			links, err := mediawiki.GetExternalLinks(mwURI, title)
			if err != nil {
				// TODO: unify the usage of errors or delete it
				errs <- fmt.Errorf("GetExternalLinks failed for %s: %v", title, err)
			}
			if len(links) == 0 {
				return
			}

			// means there are any external links on a profile page
			logger.Printf("%v discovered", title)

			// create a user and registries
			usr := user{Title: title}
			for _, link := range links {
				if strings.Contains(link, "orcid.org") {
					id, err := orcid.IDFromURL(link)
					if err != nil {
						errs <- fmt.Errorf("GetExternalLinks failed to create orcid registry: %v", err)
						break
					}
					usr.OrcID = id
					// there could be infinite amount of
					// ORCIDs on a page, but we add only
					// the first one and break
					break
				}
			}

			// return only users for whom we need to update profile pages
			if !usr.OrcID.IsEmpty() {
				mut.Lock()
				users = append(users, &usr)
				mut.Unlock()
			}
		}(title)
	}
	for i := 0; i < cap(sem); i++ {
		sem <- true
	}
	close(errs)
	close(sem)
	for err := range errs {
		if err != nil {
			return nil, errors.Wrap(err, "failed to collect an offer")
		}
	}

	return users, err
}

// updateProfilePagesWithWorks fetches works for each user, updates personal pages and
// purges cache for the aggregate Publications page.
func updateProfilePagesWithWorks(mwURI, lgName, lgPass, sectionTitle string, users []*user, logger *log.Logger) error {
	if len(users) == 0 {
		return nil
	}

	const (
		tmpl         = "publications-list.tmpl" // TODO: should be passed by a user
		contentModel = "wikitext"
	)

	for _, u := range users {
		byTypeAndYear := groupByTypeAndYear(u.Works, logger)

		markup, err := renderTmpl(byTypeAndYear, tmpl)
		if err != nil {
			return err
		}

		_, err = mediawiki.UpdatePage(mwURI, u.Title, markup, contentModel, lgName, lgPass, sectionTitle)
		if err != nil {
			logger.Printf("profile page update failed for %s with error: %v", u.Title, err)
			err = nil
			continue
		}

		logger.Printf("profile page for %s is updated", u.Title)
	}

	return nil
}

func updatePublicationsByYearWithWorks(mwURI, lgName, lgPass string, users []*user, logger *log.Logger, cref *crossref.Client) error {
	if len(users) == 0 {
		return nil
	}

	const (
		tmpl         = "publications-by-year.tmpl" // TODO: should be passed by a user
		pageTitle    = "PI_Publications_By_Year"
		contentModel = "wikitext"
		sectionTitle = "Publications By Year"
	)

	var works = []*orcid.Work{}
	for _, u := range users {
		works = append(works, u.Works...)
	}

	byTypeAndYear := groupByTypeAndYear(works, logger)

	markup, err := renderTmpl(byTypeAndYear, tmpl)
	if err != nil {
		return err
	}
	_, err = mediawiki.UpdatePage(mwURI, pageTitle, markup, contentModel, lgName, lgPass, sectionTitle)
	if err != nil {
		return err
	}

	logger.Printf("%s page has been updated", pageTitle)
	return mediawiki.Purge(mwURI, "Publications")
}

func renderTmpl(data interface{}, tmplPath string) (string, error) {
	var tmpl = template.Must(template.New("").Funcs(tmplFuncs).ParseFiles(tmplPath))
	var out bytes.Buffer
	err := tmpl.ExecuteTemplate(&out, tmplPath, data)
	return out.String(), err
}

func getYearsSorted(works []*orcid.Work) []int {
	var years = make(map[int]bool)
	for _, w := range works {
		years[w.Year] = true
	}

	var yearsSorted = []int{}
	for k := range years {
		yearsSorted = append(yearsSorted, k)
	}

	sort.Slice(yearsSorted, func(i, j int) bool {
		return yearsSorted[i] > yearsSorted[j]
	})

	return yearsSorted
}

func groupByTypeAndYear(works []*orcid.Work, logger *log.Logger) map[string][][]*orcid.Work {
	// grouping by work type
	byType := make(map[string][]*orcid.Work)
	const (
		t1 = "Journal Articles"
		t2 = "Conference Papers"
		t3 = "Other"
	)
	byType[t1] = []*orcid.Work{}
	byType[t2] = []*orcid.Work{}
	byType[t3] = []*orcid.Work{}
	for _, w := range works {
		switch w.Type {
		case "journal-article":
			byType[t1] = append(byType[t1], w)
			continue
		case "conference-paper":
			byType[t2] = append(byType[t2], w)
			continue
		default:
			byType[t3] = append(byType[t3], w)
		}
	}

	// grouping each type group by year
	byTypeAndYear := make(map[string][][]*orcid.Work)
	for k, group := range byType {
		years := getYearsSorted(group)

		if byTypeAndYear[k] == nil {
			byTypeAndYear[k] = make([][]*orcid.Work, len(years))
		}

		for i, year := range years {
			if byTypeAndYear[k][i] == nil {
				byTypeAndYear[k][i] = []*orcid.Work{}
			}

			for _, w := range group {
				if w.Year == year {
					byTypeAndYear[k][i] = append(byTypeAndYear[k][i], w)
				}
			}
		}

	}

	// removing duplicates
	for t := range byTypeAndYear {
		for i := range byTypeAndYear[t] {
			works, err := filterDuplicatedWorksByDOI(byTypeAndYear[t][i], logger)
			if err != nil {
				logger.Println(err)
				continue
			}
			byTypeAndYear[t][i] = works
		}
	}

	return byTypeAndYear
}

func stripPrefix(s, prefix string) string {
	return strings.TrimPrefix(s, prefix)
}

func stripPrefixURL(s template.HTML, prefix string) string {
	return strings.TrimPrefix(string(s), prefix)
}

func unescape(s template.HTML) (template.HTML, error) {
	u, err := url.Parse(string(s))
	if err != nil {
		return "", err
	}

	return template.HTML(u.Scheme + "://" + u.Host + u.Path), nil
}

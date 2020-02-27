package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"bitbucket.org/iharsuvorau/ims-publications/crossref"
	"bitbucket.org/iharsuvorau/ims-publications/orcid"
)

func main() {
	mwBaseURL := flag.String("mediawiki", "https://ims.ut.ee", "mediawiki base URL")
	crossrefURL := flag.String("crossref", "http://api.crossref.org/v1", "crossref API base URL")
	orcidURL := flag.String("orcid", "https://pub.orcid.org/v2.1", "orcid API base URL")
	section := flag.String("section", "Publications", "section title for the publication to look for on a user's page or of the new one to add to the page")
	category := flag.String("category", "", "category of users to update profile pages for, if it's empty all users' pages will be updated")
	lgName := flag.String("name", "", "login name of the bot for updating pages")
	lgPass := flag.String("pass", "", "login password of the bot for updating pages")
	logPath := flag.String("log", "", "specify the filepath for a log file, if it's empty all messages are logged into stdout")
	flag.Parse()

	flagsStringFatalCheck(mwBaseURL, crossrefURL, section, lgName, lgPass)

	var logger *log.Logger
	if len(*logPath) > 0 {
		f, err := os.Create(*logPath)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		logger = log.New(f, "", log.LstdFlags)
	} else {
		logger = log.New(os.Stdout, "", log.LstdFlags)
	}

	//
	// Publications for each user
	//

	// TODO: a general issue for many functions — if we pass a logger to a function, it shouldn't return an error, it should log it

	users, err := exploreUsers(*mwBaseURL, *category, logger)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Printf("users to update: %+v", len(users))

	orcidClient, err := orcid.New(*orcidURL)
	if err != nil {
		logger.Fatal(err)
	}

	if err = fetchPublicationsIfNeeded(logger, users, orcidClient); err != nil {
		logger.Fatal(err)
	}

	// crossref part
	crossrefClient, err := crossref.New(*crossrefURL)
	if err != nil {
		logger.Fatal(err)
	}
	err = fetchMissingAuthors(crossrefClient, logger, users)
	if err != nil {
		logger.Fatal(err)
	}

	removeDuplicatedWorks(users, logger)

	// saving XML for each user
	{
		var fpath string
		var err error
		var obsoleteDuration = time.Hour * 23 // TODO: time condition should be passed by a caller
		for _, u := range users {
			fpath = u.OrcID.String() + ".xml"
			if !isFileNew(fpath, obsoleteDuration) { // saves once in 23 hours
				err = dumpUserWorksXML(u)
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}

	// used by the template in updateProfilePagesWithWorks
	updateContributorsLine(users) // TODO: make cleaner, hide this detail

	err = updateProfilePagesWithWorks(*mwBaseURL, *lgName, *lgPass, *section, users, logger, crossrefClient)
	if err != nil {
		logger.Fatal(err)
	}

	//
	// Publications on the Publications page
	//

	// TODO: repetitive section of code

	usersPI, err := exploreUsers(*mwBaseURL, "PI", logger)
	if err != nil {
		log.Fatal(err)
	}
	logger.Printf("PI users to process: %+v", len(usersPI))

	if err = fetchPublicationsIfNeeded(logger, usersPI, orcidClient); err != nil {
		logger.Fatal(err)
	}

	err = fetchMissingAuthors(crossrefClient, logger, usersPI)
	if err != nil {
		logger.Fatal(err)
	}

	removeDuplicatedWorks(usersPI, logger)

	// used by the template in updateProfilePagesWithWorks
	updateContributorsLine(usersPI) // TODO: make cleaner, hide this detail

	err = updatePublicationsByYearWithWorks(*mwBaseURL, *lgName, *lgPass, usersPI, logger, crossrefClient)
	if err != nil {
		logger.Fatal(err)
	}
}

func flagsStringFatalCheck(ss ...*string) {
	for _, s := range ss {
		if len(*s) == 0 {
			log.Fatalf("fatal: flag %s has the length of zero", *s)
		}
	}
}

func dumpUserWorksXML(u *user) error {
	fpath := u.OrcID.String() + ".xml"
	f, err := os.Create(fpath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %v", fpath, err)
	}
	if err = xml.NewEncoder(f).Encode(u.Works); err != nil {
		return fmt.Errorf("failed to encode xml: %+v", err)
	}
	f.Close()
	return nil
}

func dumpUserJSON(u *user) (string, error) {
	fpath := u.OrcID.String() + ".json"
	f, err := os.Create(fpath)
	if err != nil {
		return "", fmt.Errorf("failed to create file %s: %v", fpath, err)
	}
	if err = json.NewEncoder(f).Encode(u); err != nil {
		return "", fmt.Errorf("failed to encode json: %+v", err)
	}
	f.Close()
	return fpath, nil
}

func readUserWorksXML(u *user, fpath string) error {
	f, err := os.Open(fpath)
	if err != nil {
		return err
	}
	defer f.Close()

	return xml.NewDecoder(f).Decode(&u.Works)
}

func readUserJSON(u *user, fpath string) error {
	f, err := os.Open(fpath)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewDecoder(f).Decode(&u)
}

// isFileNew checks the existence of a file and its modification time
// and returns true if it was modified during the previous maxDuration
// hours.
func isFileNew(fpath string, maxDuration time.Duration) bool {
	stat, err := os.Stat(fpath)
	if err != nil {
		return false
	}

	return time.Since(stat.ModTime()) < maxDuration
}

// updateContributorsLine populates a slice of works with an URI for
// external ids if its value is missing.
func updateContributorsLine(users []*user) {
	for _, u := range users {
		for _, w := range u.Works {
			contribs := make([]string, len(w.Contributors))
			for ii, c := range w.Contributors {
				contribs[ii] = c.Name
			}

			// formatting of contributors is according to
			// https://research.moreheadstate.edu/c.php?g=107001&p=695197
			w.ContributorsLine = strings.Join(contribs, ", ")
		}

	}

}

func fetchPublicationsIfNeeded(logger *log.Logger, users []*user, orcidClient *orcid.Client) error {
	if len(users) == 0 {
		return nil
	}

	var err error
	var fpath string
	// files become obsolete 1 hour before the next cron run
	var obsoleteDuration = time.Hour * 23 // TODO: time condition should be passed by a caller

	for _, u := range users { // TODO: use goroutines
		fpath = u.OrcID.String() + ".xml"
		if isFileNew(fpath, obsoleteDuration) {
			logger.Printf("reading from a file for %v", u.Title)
			u.Works, err = orcid.ReadWorks(fpath)
		} else {
			logger.Printf("fetching works from ORCID for %v", u.Title)
			u.Works, err = orcid.FetchWorks(orcidClient, u.OrcID, logger,
				orcid.UpdateExternalIDsURL, orcid.UpdateContributorsLine, orcid.UpdateMarkup)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func fetchMissingAuthors(cref *crossref.Client, logger *log.Logger, users []*user) error {
	logger.Println("starting crossref authors checking")
	start := time.Now()
	defer func() {
		logger.Println("crossref authors checking has been finished in", time.Since(start))
	}()

	for _, u := range users {
		for _, w := range u.Works {
			// skip if there are authors already
			if len(w.Contributors) > 0 {
				continue
			}

			w.Contributors = crossRefContributors(w, cref, logger)

			// skip if there are authors already
			if len(w.Contributors) > 0 {
				continue
			}

			w.Contributors = citationContributors(w, logger)
		}
	}

	return nil
}

func removeDuplicatedWorks(users []*user, logger *log.Logger) {
	for _, u := range users {
		if err := removeDuplicatedWorksByDOI(u, logger); err != nil {
			logger.Printf("failed to remove duplicates by DOI for %v: %v", u.OrcID, err)
		}
	}
}

func removeDuplicatedWorksByDOI(u *user, logger *log.Logger) error {
	m := make(map[string]bool)
	uniqueWorks := []*orcid.Work{}

	for _, w := range u.Works {
		if !w.HasDOI() && len(w.ExternalIDs) > 0 {
			uniqueWorks = append(uniqueWorks, w)
			continue
		}

		id := w.GetDOI()
		if id == nil {
			return fmt.Errorf("DOI must exist, but nil is returned for %v", w.Path)
		}

		if _, ok := m[id.Value]; !ok {
			m[id.Value] = true
			uniqueWorks = append(uniqueWorks, w)
		} else {
			logger.Printf("skipping a duplicate: %v", id.Value)
		}

	}

	u.Works = uniqueWorks
	return nil
}

func reportDuplicatedWorksByDOI(u *user, logger *log.Logger) (unique []string, dups []string) {
	type doi string
	m := make(map[doi]*orcid.Work)
	unique = []string{}
	dups = []string{}

	for _, w := range u.Works {
		if !w.HasDOI() && len(w.ExternalIDs) > 0 {
			unique = append(unique, w.ExternalIDs[0].Value)
			continue
		}

		id := w.GetDOI()

		if _, ok := m[doi(id.Value)]; !ok {
			m[doi(id.Value)] = w
			unique = append(unique, id.Value)
		} else {
			dups = append(dups, id.Value)
		}
	}

	logger.Printf("unique works: %v, duplicated works: %v, works originally: %v", len(unique), len(dups), len(u.Works))

	return
}

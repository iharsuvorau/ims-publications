package main

import (
	"log"
	"time"

	"bitbucket.org/iharsuvorau/ims-publications/crossref"
	"bitbucket.org/iharsuvorau/ims-publications/orcid"
)

func crossRefContributors(w *orcid.Work, cref *crossref.Client, logger *log.Logger) []*orcid.Contributor {
	if len(w.Contributors) > 0 {
		return nil
	}

	// TODO: another DOI check was introduced through the .HasDOI() and .GetDOI() methods on orcid.Work. Have to unify the procedure.

	// DOI check
	if len(string(w.DoiURI)) == 0 {
		logger.Printf("publication doesn't have DOI: %v", w.Title)
		return nil
	}

	// crossref download
	id, err := crossref.DOIFromURL(string(w.DoiURI))
	if err != nil {
		log.Println(err)
		return nil
	}

	logger.Printf("crossref fetch: %s, %s", w.Title, id)
	work, err := crossref.GetWork(cref, id)
	if err != nil {
		logger.Printf("crossref fetch error: %v", err)
		time.Sleep(time.Second * 1) // give the server time to rest
		return nil
	}

	contribs := []*orcid.Contributor{}
	for _, v := range work.Authors {
		contribs = append(contribs, &orcid.Contributor{Name: v})
	}

	return contribs
}

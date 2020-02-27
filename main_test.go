package main

import (
	"log"
	"os"
	"reflect"
	"testing"

	"bitbucket.org/iharsuvorau/ims-publications/crossref"
	"bitbucket.org/iharsuvorau/ims-publications/orcid"
)

// func TestExploreUsers(t *testing.T) {
// 	logger := log.New(os.Stdout, "", log.LstdFlags)

// 	args := []struct {
// 		name      string
// 		uri       string
// 		category  string
// 		wantErr   bool
// 		zeroUsers bool
// 	}{
// 		{
// 			name:      "A",
// 			uri:       "http://hefty.local/~ihar/ims/1.32.2",
// 			category:  "PI",
// 			wantErr:   false,
// 			zeroUsers: false,
// 		},
// 		{
// 			name:      "B",
// 			uri:       "http://hefty.local/~ihar/ims/1.32.2/",
// 			category:  "PI",
// 			wantErr:   false,
// 			zeroUsers: false,
// 		},
// 		{
// 			name:      "C",
// 			uri:       "http://hefty.local/~ihar/ims/1.32.2/",
// 			category:  "",
// 			wantErr:   false,
// 			zeroUsers: false,
// 		},
// 		{
// 			name:      "D",
// 			uri:       "http://hefty.local/",
// 			category:  "",
// 			wantErr:   true,
// 			zeroUsers: true,
// 		},
// 		{
// 			name:      "E",
// 			uri:       "http://hefty.local/",
// 			category:  "PI",
// 			wantErr:   true,
// 			zeroUsers: true,
// 		},
// 	}

// 	for _, arg := range args {
// 		t.Run(arg.name, func(t *testing.T) {
// 			users, err := exploreUsers(arg.uri, arg.category, logger)
// 			if users != nil {
// 				t.Logf("users len: %v", len(users))
// 			}
// 			if err != nil && !arg.wantErr {
// 				t.Error(err)
// 			}
// 			if users != nil && len(users) == 0 && !arg.zeroUsers {
// 				t.Errorf("amount of users must be gt 0, arg: %+v", arg)
// 			}

// 		})
// 	}
// }

func Test_groupByTypeAndYear(t *testing.T) {
	ids := []string{
		"https://orcid.org/0000-0002-1720-1509",
		"https://orcid.org/0000-0002-9151-1548",
	}

	logger := log.New(os.Stdout, "", log.LstdFlags)

	const apiBase = "https://pub.orcid.org/v2.1"

	client, err := orcid.New(apiBase)
	if err != nil {
		t.Error(err)
	}

	for _, id := range ids {
		oid, err := orcid.IDFromURL(id)
		if err != nil {
			t.Error(err)
		}

		works, err := orcid.FetchWorks(client, oid, logger)
		if err != nil {
			t.Error(err)
		}

		if len(works) == 0 {
			t.Error("amount of works must be bigger than zero")
		}

		byTypeAndYear := groupByTypeAndYear(works)
		//t.Logf("result: %+v", byTypeAndYear)

		markup, err := renderTmpl(byTypeAndYear, "publications-by-year.tmpl")
		if err != nil {
			t.Error(err)
		}
		if len(markup) == 0 {
			t.Error("there must be markup, got 0")
		}
		//t.Logf("markup: %s", markup)
	}
}

func Test_getMissingAuthorsCrossRef(t *testing.T) {
	ids := []string{
		"https://orcid.org/0000-0001-8221-9820",
	}

	logger := log.New(os.Stdout, "", log.LstdFlags)

	cref, err := crossref.New("http://api.crossref.org/v1")
	if err != nil {
		t.Fatal(err)
	}

	const apiBase = "https://pub.orcid.org/v2.1"

	orcl, err := orcid.New(apiBase)
	if err != nil {
		t.Error(err)
	}

	for _, id := range ids {
		oid, err := orcid.IDFromURL(id)
		if err != nil {
			t.Error(err)
		}

		works, err := orcid.FetchWorks(orcl, oid, logger)
		if err != nil {
			t.Fatal(err)
		}

		if len(works) == 0 {
			t.Error("amount of works must be bigger than zero")
		}

		t.Logf("contributors before: %+v", works[0].Contributors)

		works[0].Contributors = crossRefContributors(works[0], cref, logger)

		t.Logf("contributors after: %+v", works[0].Contributors)
	}
}

func Test_fetchPublicationsAndMissingAuthors(t *testing.T) {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	const mwBaseURL = "https://ims.ut.ee"
	const category = "PI"
	const crossrefURL = "http://api.crossref.org/v1"

	users, err := exploreUsers(mwBaseURL, category, logger)
	if err != nil {
		t.Error(err)
	}

	const apiBase = "https://pub.orcid.org/v2.1"

	orcl, err := orcid.New(apiBase)
	if err != nil {
		t.Error(err)
	}

	err = fetchPublicationsIfNeeded(logger, users, orcl)
	if err != nil {
		t.Fatal(err)
	}

	for _, u := range users {
		if l := len(u.Works); l == 0 {
			t.Errorf("want more works, have %v", l)
		}
	}

	// limit the number of users and works
	if len(users) > 1 {
		users = users[:1]
	}
	if len(users[0].Works) > 2 {
		users[0].Works = users[0].Works[:2]
	}

	t.Log("before")
	for _, u := range users {
		t.Log(u.Title)
		for _, w := range u.Works {
			t.Log(w.Title)
			t.Logf("authors: %+v", w.Contributors)
		}
	}

	cref, err := crossref.New(crossrefURL)
	if err != nil {
		log.Fatal(err)
	}

	err = fetchMissingAuthors(cref, logger, users)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("after")
	for _, u := range users {
		t.Log(u.Title)
		for _, w := range u.Works {
			t.Log(w.Title)
			for _, v := range w.Contributors {
				t.Logf("\tauthor: %+v", v)
			}
		}
	}
}

func Test_parseCitationAuthorsIEEE(t *testing.T) {
	tests := []struct {
		name     string
		citation string
		result   string
		wantErr  bool
	}{
		{
			name:     "A",
			citation: "Saoni Banerji, R.Senthil Kumar (2010). Diagnosis of Systems Via Condition Monitoring Based on Time Frequency Representations. International Journal of Recent Trends in Engineering & Research, 4 (2), 20−24.01.IJRTET 04.02.102.",
			result:   "Saoni Banerji, R.Senthil Kumar",
			wantErr:  false,
		},
		{
			name:     "B",
			citation: `S. Banerji, J. Madrenas and D. Fernandez, "Optimization of parameters for CMOS MEMS resonant pressure sensors," 2015 Symposium on Design, Test, Integration and Packaging of MEMS/MOEMS (DTIP), Montpellier, 2015, pp. 1-6. doi: 10.1109/DTIP.2015.7160984`,
			result:   "S. Banerji, J. Madrenas and D. Fernandez",
			wantErr:  false,
		},
		{
			name:     "C",
			citation: ` @phdthesis{banerji2012ultrasonic, title= {Ultrasonic Link IC for Wireless Power and Data Transfer Deep in Body}, author= {Banerji, Saoni and Ling, Goh Wang and Cheong, Jia Hao and Je, Minkyu}, year= {2012}, school= {Nanyang Technological University}} `,
			result:   "Banerji, Saoni and Ling, Goh Wang and Cheong, Jia Hao and Je, Minkyu",
			wantErr:  false,
		},
		{
			name:     "D",
			citation: `Banerji, Saoni & Chiva, Josep. (2016). Under pressure? Do not lose direction! Smart sensors: Development of MEMS and CMOS on the same platform.`,
			result:   "Banerji, Saoni & Chiva, Josep",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := parseCitationAuthorsIEEE(tt.citation)
			if err != nil && !tt.wantErr {
				t.Error(err)
			}
			if !reflect.DeepEqual(res, tt.result) {
				t.Errorf("want %q, got %q", tt.result, res)
			}
		})
	}
}

func Test_parseCitationAuthorsBibTeX(t *testing.T) {
	tests := []struct {
		name     string
		citation string
		result   string
		wantErr  bool
	}{
		{
			name: "A",
			citation: `S. Banerji, W. L. Goh, J. H. Cheong and M. Je, "CMUT ultrasonic power link front-end for wireless power transfer deep in body," 2013 IEEE MTT-S International Microwave Workshop Series on RF and Wireless Technologies for Biomedical and Healthcare Applications (IMWS-BIO), Singapore, 2013, pp. 1-3.
doi: 10.1109/IMWS-BIO.2013.6756176`,
			result:  "S. Banerji, W. L. Goh, J. H. Cheong and M. Je",
			wantErr: false,
		},
		{
			name:     "B",
			citation: `@inproceedings{Vunder_2018,doi = {10.1109/hsi.2018.8431062},url = {https://doi.org/10.1109%2Fhsi.2018.8431062},year = 2018,month = {jul},publisher = {{IEEE}},author = {Veiko Vunder and Robert Valner and Conor McMahon and Karl Kruusamae and Mitch Pryor},title = {Improved Situational Awareness in {ROS} Using Panospheric Vision and Virtual Reality},booktitle = {2018 11th International Conference on Human System Interaction ({HSI})}}`,
			result:   "Veiko Vunder and Robert Valner and Conor McMahon and Karl Kruusamae and Mitch Pryor",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := parseCitationAuthorsBibTeX(tt.citation)
			if err != nil && !tt.wantErr {
				t.Error(err)
			}
			if !reflect.DeepEqual(res, tt.result) {
				t.Errorf("want %q, got %q", tt.result, res)
			}
		})
	}
}

func Test_parseCitationAuthorsBibTeXStrict(t *testing.T) {
	tests := []struct {
		name     string
		citation string
		result   string
		wantErr  bool
	}{
		{
			name: "A",
			citation: `S. Banerji, W. L. Goh, J. H. Cheong and M. Je, "CMUT ultrasonic power link front-end for wireless power transfer deep in body," 2013 IEEE MTT-S International Microwave Workshop Series on RF and Wireless Technologies for Biomedical and Healthcare Applications (IMWS-BIO), Singapore, 2013, pp. 1-3.
doi: 10.1109/IMWS-BIO.2013.6756176`,
			result:  "",
			wantErr: true,
		},
		{
			name:     "B",
			citation: `@inproceedings{Vunder_2018,doi = {10.1109/hsi.2018.8431062},url = {https://doi.org/10.1109%2Fhsi.2018.8431062},year = 2018,month = {jul},publisher = {{IEEE}},author = {Veiko Vunder and Robert Valner and Conor McMahon and Karl Kruusamae and Mitch Pryor},title = {Improved Situational Awareness in {ROS} Using Panospheric Vision and Virtual Reality},booktitle = {2018 11th International Conference on Human System Interaction ({HSI})}}`,
			result:   "Veiko Vunder and Robert Valner and Conor McMahon and Karl Kruusamae and Mitch Pryor",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := parseCitationAuthorsBibTeXStrict(tt.citation)
			if err != nil && !tt.wantErr {
				t.Error("parsing failed:", err)
			}
			if !reflect.DeepEqual(res, tt.result) {
				t.Errorf("want %q, got %q", tt.result, res)
			}
		})
	}
}

func Test_unescapedDoiURL(t *testing.T) {
	ids := []string{
		"https://orcid.org/0000-0003-0466-2514",
	}

	logger := log.New(os.Stdout, "", log.LstdFlags)

	const apiBase = "https://pub.orcid.org/v2.1"

	users, err := exploreUsers("https://ims.ut.ee", "PI", logger)
	if err != nil {
		t.Error(err)
	}

	t.Logf("users: %v", len(users))

	filteredUsers := []*user{}
	for _, id := range ids {
		oid, err := orcid.IDFromURL(id)
		if err != nil {
			t.Error(err)
		}

		for _, u := range users {
			if string(u.OrcID) == string(oid) {
				filteredUsers = append(filteredUsers, u)
			}
		}
	}

	t.Logf("filtered users: %v", len(filteredUsers))

	orcidClient, err := orcid.New(apiBase)
	if err != nil {
		t.Error(err)
	}

	for _, u := range filteredUsers {
		u.Works, err = orcid.FetchWorks(orcidClient, u.OrcID, logger,
			orcid.UpdateExternalIDsURL, orcid.UpdateContributorsLine, orcid.UpdateMarkup)
		if err != nil {
			t.Error(err)
		}
	}

	updateContributorsLine(filteredUsers)

	for _, u := range filteredUsers {
		byTypeAndYear := groupByTypeAndYear(u.Works)

		markup, err := renderTmpl(byTypeAndYear, "publications-list.tmpl")
		if err != nil {
			t.Error(err)
		}

		t.Log(markup)
	}
}

func Test_removeDuplicatedWorksByDOI(t *testing.T) {
	// setup

	fpath := "testdata/0000-0003-0466-2514.json"
	logger := log.New(os.Stdout, "", log.LstdFlags)

	u := &user{}

	err := readUserJSON(u, fpath)
	if err != nil {
		t.Fatal(err)
	}

	if len(u.Works) == 0 {
		t.Fatal("need more works for a test")
	}

	logger.Printf("%v works were read", len(u.Works))

	// test

	type args struct {
		u      *user
		logger *log.Logger
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "A",
			args: args{
				u:      u,
				logger: logger,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worksBefore := len(tt.args.u.Works)

			err := removeDuplicatedWorksByDOI(tt.args.u, tt.args.logger)
			if err != nil && !tt.wantErr {
				t.Fatal(err)
			}

			worksAfter := len(tt.args.u.Works)

			if worksBefore != 91 {
				t.Fatal("wrong number of original works")
			}

			if worksAfter != 83 {
				t.Fatalf("wrong number of unique works, got %v, want %v", worksAfter, 83)
			}

			t.Logf("before: %v, after: %v", worksBefore, worksAfter)
		})
	}
}

func Test_reportDuplicatedWorksByDOI(t *testing.T) {
	// setup

	fpath := "testdata/0000-0003-0466-2514.json"
	logger := log.New(os.Stdout, "", log.LstdFlags)

	u := &user{}

	err := readUserJSON(u, fpath)
	if err != nil {
		t.Fatal(err)
	}

	if len(u.Works) == 0 {
		t.Fatal("need more works for a test")
	}

	logger.Printf("%v works were read", len(u.Works))

	// test

	type args struct {
		u      *user
		logger *log.Logger
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "A",
			args: args{
				u:      u,
				logger: logger,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unique, dups := reportDuplicatedWorksByDOI(tt.args.u, tt.args.logger)
			if len(unique) != 83 || len(dups) != 8 {
				t.Fatalf("got %v unique, want %v and %v dups, want %v",
					len(unique), 83, len(dups), 8)
			}
		})
	}
}

func Test_dumpUserJSON_and_readUserJSON(t *testing.T) {
	// Note: This test fetches real data from ORCID API.

	// setup

	logger := log.New(os.Stdout, "", log.LstdFlags)
	mwURI := "https://ims.ut.ee"
	orcidURL := "https://pub.orcid.org/v2.1"
	category := "PI"
	tarmoOrcID := orcid.ID("0000-0003-0466-2514")

	users, err := exploreUsers(mwURI, category, logger)
	if err != nil {
		t.Fatal(err)
	}

	// filter out everybody except the test subject
	usersFiltered := make([]*user, 1)
	for _, v := range users {
		if v.OrcID == tarmoOrcID {
			usersFiltered[0] = v
			break
		}
	}

	// fetch publications
	orcidClient, err := orcid.New(orcidURL)
	if err != nil {
		t.Fatal(err)
	}
	if err = fetchPublicationsIfNeeded(logger, usersFiltered, orcidClient); err != nil {
		t.Fatal(err)
	}

	// getting the test subject
	u := usersFiltered[0]
	if u == nil {
		t.Fatal("Tarmo wasn't found")
	}

	// test

	// TODO: test shouldn't create persistent output files, remove them at the end

	fpath, err := dumpUserJSON(u)
	if err != nil {
		t.Fatal(err)
	}

	uu := new(user)
	err = readUserJSON(uu, fpath)

	if u.Title != uu.Title || u.OrcID != uu.OrcID || len(u.Works) != len(uu.Works) {
		t.Fatal("dumped and read back data is different")
	}
}

**publications-update** downloads scientific publications from [orcid.org](https://orcid.org/) and [crossref.org](https://www.crossref.org/), and updates MediaWiki pages.

To quickly deploy to the server, update `Makefile` for your server location, binary and templates destinations, then run (assuming SSH is up and configured):

```
$ make deploy
```

Run it on the server like this:

```
$ publications-update -mwuri https://ims.ut.ee/ -name "UserName" -pass "pass" -log "publications.log"
```

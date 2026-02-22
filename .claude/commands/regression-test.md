Run the CrystalMUSH regression test suite. This includes both out-of-game (Go unit tests) and in-game (WebSocket smoke tests).

Steps:
1. Run the full Go unit test suite via Docker:
   `docker run --rm -v "/mnt/f/goTinyMush:/src" -w /src golang:latest sh -c "go test -buildvcs=false ./... -count=1"`

2. Deploy to CrystalMUSH test server:
   - Copy help file: `cp /mnt/f/goTinyMush/data/text/help.txt /mnt/f/CrystalMUSH/data/text/help.txt`
   - IMPORTANT: Use `docker compose up -d` not `docker compose restart` to pick up new image:
     `cd /mnt/f/CrystalMUSH && docker compose build crystalmush && docker compose up -d crystalmush`
   - Wait 8 seconds for server to start

3. Get a login token for in-game testing (Raimier is a wizard):
   `TOKEN=$(curl -s http://localhost:8443/api/v1/auth/login -H 'Content-Type: application/json' -d '{"name":"Raimier","password":"mne8994"}' | grep -oP '"token":"\K[^"]+')`

4. Run the in-game regression test from the project root:
   `cd /mnt/f/goTinyMush && TOKEN=$TOKEN node regression_test.js`

5. Report results: summarize pass/fail counts from both test runs.

Notes:
- The Docker container builds from goTinyMush source (not a pre-built binary)
- `docker compose restart` reuses the old image — always use `docker compose up -d` after build
- Login API endpoint is `/api/v1/auth/login` with field `name` (not `username`)
- The regression_test.js script connects via WebSocket with JSON protocol and JWT token auth

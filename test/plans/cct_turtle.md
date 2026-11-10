# Test plan: cct_turtle

## cct_turtle_api.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| CT-01 | GetTurtleCode | Happy Path | the startup script is requested | the startup script is embedded in the server and is not empty | Status 200; the body equals the startup script byte for byte | P1 |  |
| CT-02 | GetTurtleUpdatingCode | Happy Path | the updating startup script is requested | the updating startup script is embedded in the server and is not empty | Status 200; the body equals the updating startup script byte for byte | P1 |  |

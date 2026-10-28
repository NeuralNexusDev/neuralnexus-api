# Test plan: cct_turtle

## cct_turtle_api.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| CT-01 | GetTurtleCode | Happy Path | the startup script is requested | `cct_turtle/startup.lua` exists in the embedded public file system and is non-empty | Status 200; the body equals the bytes of `cct_turtle/startup.lua` in `public.FS` | P1 |  |
| CT-02 | GetTurtleUpdatingCode | Happy Path | the updating startup script is requested | `cct_turtle/updating_startup.lua` exists in the embedded public file system and is non-empty | Status 200; the body equals the bytes of `cct_turtle/updating_startup.lua` in `public.FS` | P1 |  |

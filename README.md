## App-Level Variables (Phase 2)
Set \`APP_ENV=prod APP_COMPANY_NAME=AcmeCorp APP_API_KEY=sk-...\`
Use in YAML: \`prompt: \"Welcome to {{.App.CompanyName}} ({{.App.Environment}})!\"\` or \`{{.Env.API_KEY}}\`
Loads at startup; templates validated on machine load. Defaults/missing: empty/fallback.
Examples: registry/yaml/app-example-v1.0.yaml, registry/yaml/company-demo-v1.0.yaml
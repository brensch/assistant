## Build

To build on github, push a git tag starting with 'v'

```
docker build -t duckdb-go-app .
```

## Run
```
docker run -d \
  --name discord-bot-production \
  -v $(pwd)/.conf:/app/.conf:ro \
  -v $(pwd)/dbfiles:/app/dbfiles \
  duckdb-go-app
```
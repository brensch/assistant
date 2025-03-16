## Build
This gets built automatically on deploy to github.

## Run
```
docker run -d \
  --name discord-bot-production \
  -v $(pwd)/.conf:/app/.conf:ro \
  -v $(pwd)/dbfiles:/app/dbfiles \
  ghcr.io/brensch/assistant
```
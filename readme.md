## Build
This gets built automatically on deploy to github.

If you're impatient, run this:

```
docker build -t assistant .

docker run -d \
  --name discord-bot-local \
  -v $(pwd)/.conf:/app/.conf:ro \
  -v $(pwd)/dbfiles:/app/dbfiles \
  assistant
```


## Run from cloud build
```
docker pull ghcr.io/brensch/assistant:latest

docker run -d \
  --name discord-bot-production \
  -v $(pwd)/.conf:/app/.conf:ro \
  -v $(pwd)/dbfiles:/app/dbfiles \
  ghcr.io/brensch/assistant
```
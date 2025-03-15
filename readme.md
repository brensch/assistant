```
gcloud functions deploy discordHandler \
  --runtime go123 \
  --trigger-http \
  --allow-unauthenticated \
  --entry-point=testDiscord

```
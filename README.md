# Chatbot

## i18n

1. `goi18n extract` to update `active.en.toml`
2. `goi18n merge active.*.toml` to generate `translate.*.toml`
3. fill up with translated words
4. `goi18n merge active.*.toml translate.*.toml` to merge all changes to active messages
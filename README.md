# calendar-mixi2-bot

Googleカレンダーの翌日の予定を取得し、mixi2 に自動投稿するBotです。

## 機能

- 複数のGoogleカレンダーを読み込み
- 翌日の予定だけを取得
- 予定がある日だけmixi2へ投稿
- 同じ日に二重投稿しないよう `state.json` で管理
- GitHub Actionsで毎日自動実行

## 自動投稿の時間

GitHub Actionsで毎日実行します。

```yaml
cron: '7 21 * * *'
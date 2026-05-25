# calendar-mixi2-bot

Googleカレンダーの翌日の予定を取得し、mixi2 に自動投稿するBotです。

## 機能

- 複数のGoogleカレンダーを読み込み
- 翌日の予定だけを取得
- 予定がある日だけmixi2へ投稿
- 同じ日に二重投稿しないよう `state.json` で管理
- GitHub Actionsで毎日自動実行
- workflow の同時実行防止あり

## 自動投稿時間

GitHub Actionsで毎日自動実行します。

```yaml
cron: '7 21 * * *'

```

## 必要な Secrets

GitHub Secrets に以下を登録してください。

- `GOOGLE_API_KEY`
- `CLIENT_ID`
- `CLIENT_SECRET`

## state.json

投稿済み日付を保存します。

```json
{
  "last_post_date": "2026-05-24"
}
```

## 実行方法

ローカル実行：

```bash
go run .
```

## プレビュー実行

投稿せずに内容だけ確認したい場合:

```bash
PREVIEW=1 go run .
```

---

GitHub Actionsからも自動実行されます。

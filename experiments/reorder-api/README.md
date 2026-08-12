# 並び替えAPI

`id` で指定した要素を、`previousId` で指定した要素の直後へ移動するAPIです。

```http
PATCH /items/{id}/position
Content-Type: application/json

{
  "previousId": "item-2"
}
```

- `id`: 移動する要素
- `previousId`: 移動後に直前となる要素
- `previousId: null`: 先頭へ移動

## 実行

リポジトリのルートで実行します。

```sh
go test ./experiments/reorder-api/...
go run ./experiments/reorder-api
```

サーバー起動後は次のコマンドで確認できます。

```sh
curl http://localhost:8080/items

curl -X PATCH http://localhost:8080/items/D/position \
  -H 'Content-Type: application/json' \
  -d '{"previousId":"A"}'
```

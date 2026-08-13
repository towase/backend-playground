# 並び替えAPI

`id` で指定した要素を、`previousId` で指定した要素の直後へ移動するAPIです。

SQLiteへ並び順を保存し、取得・検証・更新を1つのトランザクション内で行います。比較用としてメモリストアも残しています。

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

データはリポジトリ直下の `reorder.db` に保存されるため、サーバーを再起動しても並び順が残ります。保存先は環境変数で変更できます。

```sh
DATABASE_PATH=/tmp/reorder.db PORT=18080 go run ./experiments/reorder-api
```

`PORT` を省略した場合は `8080` を使います。

サーバー起動後は次のコマンドで確認できます。

```sh
curl http://localhost:8080/items

curl -X PATCH http://localhost:8080/items/D/position \
  -H 'Content-Type: application/json' \
  -d '{"previousId":"A"}'
```

## DBでの更新手順

`position` にはUNIQUE制約があります。そのまま値を入れ替えると更新途中で重複するため、次の処理を1つのトランザクション内で行います。

1. `position` 順に全件取得する
2. メモリ上で移動後の順序を計算する
3. DB上の全 `position` を一時的に負数へ変更する
4. 新しい `position` を `1` から順に更新する
5. すべて成功した場合だけコミットする

途中の更新に失敗した場合はロールバックされ、元の並び順が維持されます。

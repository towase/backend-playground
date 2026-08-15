# 間隔付き整数による並び替えAPI

整数の `position` にあらかじめ間隔を空け、通常は移動対象の1行だけを更新する方式を試します。

## API

```http
PATCH /items/{id}/position
Content-Type: application/json

{
  "previousId": "item-2"
}
```

## 確かめること

- 前後の `position` から中間値を求める
- 整数の隙間がなくなった場合に再採番する
- 通常時と再採番時の更新行数を比較する
- 同じ位置を同時更新した場合の競合を確認する

## 実行

HTTPサーバー、SQLite、初期データ、一覧取得、並び替えを実装しています。

```sh
go test ./experiments/reorder/gap-position/...
go run ./experiments/reorder/gap-position
```

```sh
curl http://localhost:8080/items

curl -X PATCH http://localhost:8080/items/D/position \
  -H 'Content-Type: application/json' \
  -d '{"previousId":"A"}'
```

## 更新方法

移動対象を現在の並びから取り除き、移動後の直前・直後にある要素の `position` を調べます。

- 隙間がある場合: 中間値を求めて移動対象の1行だけ更新する
- 末尾へ移動する場合: 直前の `position + 100` を使用する
- 隙間がない場合: 全要素を移動後の順番で `100, 200, 300...` に再採番する

再採番はトランザクション内で行い、UNIQUE制約との一時的な衝突を避けるため、現在のpositionを負数へ退避してから新しい値を割り当てます。

## 同時更新の確認

通常のストアはSQLiteへの接続数を1つに制限しているため、APIから同時に並び替えても処理は直列化されます。

競合そのものを観察するテストでは接続を2つ確保し、2つのトランザクションが同じ初期状態を読み取ってから、DとEをそれぞれAの直後へ移動します。どちらも `position = 150` を算出しますが、先に更新したトランザクションだけがコミットされ、もう一方はSQLiteに拒否されます。

```sh
go test -run 'TestConcurrentReorders|TestTwoTransactions' -v ./experiments/reorder/gap-position/internal/item
```

競合した処理を成功させるには、失敗したトランザクションを最初から再実行し、最新の並びを読み直して新しいpositionを計算する必要があります。

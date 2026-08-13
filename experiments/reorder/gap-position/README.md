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

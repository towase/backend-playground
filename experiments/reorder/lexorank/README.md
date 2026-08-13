# LexoRankによる並び替えAPI

辞書順で比較できる文字列の `rank` を使い、前後のrankの間に新しいrankを生成する方式を試します。

## API

```http
PATCH /items/{id}/position
Content-Type: application/json

{
  "previousId": "item-2"
}
```

## 確かめること

- 前後のrankから中間のrankを生成する
- 同じ位置への挿入を繰り返した場合のrank変化を確認する
- rankが偏った場合に平準化する
- 同時更新時のrank重複を扱う

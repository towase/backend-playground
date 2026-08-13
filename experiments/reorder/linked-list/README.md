# 連結リストによる並び替えAPI

各要素に `previous_id` を持たせ、リンクをつなぎ替えて並び順を表現する方式を試します。

## API

```http
PATCH /items/{id}/position
Content-Type: application/json

{
  "previousId": "item-2"
}
```

## 確かめること

- 移動時に必要なリンクをつなぎ替える
- 再帰CTEで先頭から順番に取得する
- 循環や分岐などの不正な構造を防ぐ
- 更新コストと一覧取得コストを比較する

# Backend Playground

バックエンドの仕組みを、実装と実験を通して確かめるためのPlaygroundです。

## 構成

```text
backend-playground/
├── experiments/  # 学習テーマごとの実装とテスト
├── notes/        # 自分の言葉で書く学習ログ
├── go.mod
└── README.md
```

実験は `experiments/<テーマ名>/` に追加します。それぞれの目的、起動方法、確認方法は、実験内のREADMEに記載します。

## Experiments

- [並び替え](experiments/reorder/README.md)

## 共通コマンド

すべての実験のテストを実行します。

```sh
go test ./...
```

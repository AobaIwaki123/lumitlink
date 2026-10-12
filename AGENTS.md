# lumipage 規約

このワークスペースは、Lit.linkの公開プロフィールをブラウザなしで取得し、JSON APIとして提供するGoサービス `lumipage` の実装・検証を管理します。

## 方針

- 取得は固定ホスト `lit.link` へのHTTPリクエストだけで行い、ブラウザ、Playwright、Selenium、JavaScript実行環境に依存しない。
- Lit.linkの公開HTMLに含まれるメタデータとNext.jsのサーバー側データを、`internal/litlink` のアダプター内で解析する。
- 外部ページの仕様変更を前提に、キャッシュ、取得サイズ上限、タイムアウト、取得結果の観測情報を持たせる。
- Lit.linkへの負荷を抑えるため、リクエストごとの再取得を避ける。
- 公開データの読み取り専用とし、ログイン、編集、認証情報の取得は扱わない。
- APIの既定プロフィールは `amamiyasaku` だが、取得経路は同じ形式の公開プロフィールを検証できるよう `{username}` 形式を維持する。
- UIはデバッグ可能性を優先し、HTTPステータス、API処理時間、キャッシュ状態、生JSONを表示する。

## 検証

```bash
cd space/lumipage
go test ./...
go run ./cmd/lumipage
curl -i http://127.0.0.1:8080/healthz
curl -i http://127.0.0.1:8080/api/v1/profiles/amamiyasaku
```

サーバーの標準エラー出力には、HTTPメソッド、パス、ステータス、処理時間、キャッシュ状態をJSON Lines形式で出力します。Kubernetesでは次のコマンドで実ログを確認します。

```bash
kubectl logs -f deployment/lumipage -n lumipage
```

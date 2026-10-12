# lumipage

Lit.linkの公開プロフィールを、ブラウザなしで取得してJSON APIとして提供する小さなGoサーバーです。lumiシリーズのプロフィール取得担当として、まず `amamiyasaku` を初期表示対象にしています。

## 起動

```bash
go run ./cmd/lumipage
```

環境変数:

| 変数 | デフォルト | 用途 |
| :--- | :--- | :--- |
| `LUMIPAGE_HOST` | `0.0.0.0` | 待受アドレス |
| `LUMIPAGE_PORT` | `8080` | 待受ポート |
| `LUMIPAGE_CACHE_TTL` | `10m` | インメモリキャッシュ期間 |
| `LUMIPAGE_TIMEOUT` | `10s` | Lit.linkへのHTTPタイムアウト |
| `LUMIPAGE_USER_AGENT` | `lumipage/0.1 (+https://lit.link/)` | upstreamへ送るUser-Agent |
| `LOG_LEVEL` | `INFO` | ログレベル |

## API

```bash
curl -i http://127.0.0.1:8080/healthz
curl -i http://127.0.0.1:8080/api/v1/profiles/amamiyasaku
curl -i http://127.0.0.1:8080/api/v1/profiles/amamiyasaku?refresh=1
```

`GET /api/v1/profiles/{username}` は次を返します。互換用に `/v1/profiles/{username}` も受け付けます。

- `profile.name`, `profile.catchphrase`, `profile.profile_text`
- SNS、ボタン、画像、動画、音楽、ショップ、カレンダーのリンク一覧
- `metadata` のtitle、description、Open Graph、Twitter Card情報
- `source` の取得URL、HTTPステータス、レスポンスサイズ、ETag、処理時間、キャッシュ状態

## UIとログ

`http://127.0.0.1:8080/` に簡素な確認UIがあります。画面にはAPIレスポンスのHTTPステータス、処理時間、`X-Cache`、上流レスポンスサイズ、生JSONを表示します。

標準エラー出力にはJSON Lines形式のリクエストログを出します。別ターミナルから次のように監視できます。

```bash
go run ./cmd/lumipage 2>lumipage.log
tail -f lumipage.log
```

Kubernetesでは次を使います。

```bash
kubectl logs -f deployment/lumipage -n lumipage
```

## テスト

```bash
./scripts/verify-all.sh
```

テストは固定したSSR風HTMLを使うため、テスト実行時にブラウザも外部ネットワークも必要ありません。

## Kubernetes / GitHub Actions

GitHub Actionsが `main` または `v*` タグへのpushで、amd64/arm64イメージをGHCRへ自動公開します。`main` のイメージ公開後は、Kustomizeのタグをmainへ自動反映します。

```bash
kubectl apply -k k8s/manifests
kubectl rollout status deployment/lumipage -n lumipage --timeout=120s
kubectl logs -f deployment/lumipage -n lumipage
```

Ingressは `https://lumipage.aooba.net` を想定しています。ArgoCD用定義は `k8s/argocd/app.yaml` にあります。

## lumi-exporterの名寄せ方針

イベントの統合はタイトルだけで確定させず、正規化タイトル、開催日、会場、出演者、各ソースURLを候補として残します。相対評価で最高スコアを採用しますが、1位が同点、または2位との差が設定したマージン未満の場合は `ambiguous` としてアラートを上げ、自動統合しません。アラートには候補、各スコア、総合スコア、入力ソースを保存し、重みと正規化コードを見直せるようにします。

## 制約

Lit.linkの公開HTMLとNext.jsサーバー側データ形式に依存します。外部サービスの変更に備え、取得サイズ、タイムアウト、キャッシュ、構造化ログを設定しています。ログイン済み情報や編集機能は対象外です。

# lumitlink

Lit.linkの公開プロフィールを、ブラウザなしで取得してJSON APIとして提供する小さなGoサーバーです。lumiシリーズのプロフィール取得担当として、まず `amamiyasaku` を初期表示対象にしています。

## 起動

```bash
go run ./cmd/lumitlink
```

環境変数:

| 変数 | デフォルト | 用途 |
| :--- | :--- | :--- |
| `LUMITLINK_HOST` | `0.0.0.0` | 待受アドレス |
| `LUMITLINK_PORT` | `8080` | 待受ポート |
| `LUMITLINK_CACHE_TTL` | `10m` | インメモリキャッシュ期間 |
| `LUMITLINK_TIMEOUT` | `10s` | Lit.linkへのHTTPタイムアウト |
| `LUMITLINK_USER_AGENT` | `lumitlink/0.1 (+https://lit.link/)` | upstreamへ送るUser-Agent |
| `LOG_LEVEL` | `INFO` | ログレベル |

## API

```bash
curl -i http://127.0.0.1:8080/healthz
curl -i http://127.0.0.1:8080/api/v1/profiles/amamiyasaku
curl -i http://127.0.0.1:8080/api/v1/entities/amamiyasaku
curl -i http://127.0.0.1:8080/api/v1/profiles/amamiyasaku?refresh=1
```

`GET /api/v1/profiles/{username}` は次を返します。互換用に `/v1/profiles/{username}` も受け付けます。

- `profile.name`, `profile.catchphrase`, `profile.profile_text`
- SNS、ボタン、画像、動画、音楽、ショップ、カレンダーのリンク一覧
- `metadata` のtitle、description、Open Graph、Twitter Card情報
- `source` の取得URL、HTTPステータス、レスポンスサイズ、ETag、処理時間、キャッシュ状態

`GET /api/v1/entities/{username}` はプロフィールの安定した共通部分を [`lumi-spec`](https://github.com/AobaIwaki123/lumi-spec) の `entity.v1` として返します。アイコンは`assets`、XやInstagramなどのSNSは`social_links`、Lit.linkの出所は`source_refs`に入ります。

## UIとログ

`http://127.0.0.1:8080/` に簡素な確認UIがあります。画面にはAPIレスポンスのHTTPステータス、処理時間、`X-Cache`、上流レスポンスサイズ、生JSONを表示します。

標準エラー出力にはJSON Lines形式のリクエストログを出します。別ターミナルから次のように監視できます。

```bash
go run ./cmd/lumitlink 2>lumitlink.log
tail -f lumitlink.log
```

Kubernetesでは次を使います。

```bash
kubectl logs -f deployment/lumitlink -n lumitlink
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
kubectl rollout status deployment/lumitlink -n lumitlink --timeout=120s
kubectl logs -f deployment/lumitlink -n lumitlink
```

Ingressは `https://lumitlink.aooba.net` を想定しています。ArgoCD用定義は `k8s/argocd/app.yaml` にあります。

既に旧 `lumipage` Applicationを登録している場合は、新しいApplicationを同期してから旧Applicationを削除します。

```bash
kubectl apply --validate=false -f k8s/argocd/app.yaml
kubectl delete application lumipage -n argocd
```

## lumi-exporterとのpull連携

`lumi-exporter` は登録したlumiサービスをpullし、`entity.v1`、`event-feed.v1`などの共通データとして描画できます。イベントの名寄せルールと評価fixtureは [lumi-exporter](https://github.com/AobaIwaki123/lumi-exporter) 側で管理します。

## 制約

Lit.linkの公開HTMLとNext.jsサーバー側データ形式に依存します。外部サービスの変更に備え、取得サイズ、タイムアウト、キャッシュ、構造化ログを設定しています。ログイン済み情報や編集機能は対象外です。

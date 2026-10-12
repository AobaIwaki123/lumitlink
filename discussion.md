# lumitlink ディスカッション & 進捗管理

## 目的

`https://lit.link/amamiyasaku` の公開プロフィールを、ブラウザを起動せずにHTTPクライアントだけで取得し、他のアプリケーションから利用できるJSON APIとして提供する。プロジェクト名は `lit.link` に直接結びつく `lumitlink` とする。

## 方針

- Lit.linkの公開ページを固定ホストへHTTP GETする。
- SSR HTMLに含まれるメタ情報とNext.js Flightデータから、プロフィール、リンク、画像、動画、音楽、ショップ、カレンダー情報を抽出する。
- 取得結果はインメモリキャッシュし、短時間の連続アクセスでLit.linkへ負荷をかけない。
- 非公式のHTML構造に依存するため、解析失敗時にはHTTPエラーと取得ログで原因を確認できるようにする。
- APIは汎用の `/api/v1/profiles/{username}` とし、UIの初期値と運用監視対象を `amamiyasaku` に固定する。

## ロードマップ

| Step | 項目 | 内容 | 状況 |
| :--- | :--- | :--- | :--- |
| 1 | HTTP取得検証 | ブラウザなしのcurlで公開HTMLを取得できることを確認 | 完了 |
| 2 | Lit.linkアダプター | SSRデータ、メタ情報、フォールバックリンクの解析をGoで実装 | 完了 |
| 3 | JSON API | `/healthz` と `/api/v1/profiles/{username}` を提供 | 完了 |
| 4 | テスト | 固定HTMLによる解析テスト、race検査、go vet | 完了 |
| 5 | 運用強化 | Docker/Kubernetesマニフェスト、UI、構造化ログ | 完了 |
| 6 | ライブ適用 | イメージ公開、Kubernetes apply、Ingress疎通確認 | 進行中 |
| 7 | 共通entity endpoint | `/api/v1/entities/{username}`でlumi-specへ投影 | 実装中 |
| 8 | lumi-exporter | イベント名寄せと相対評価アラート | 別repoでPoC中 |

## 直近のネクストアクション

- [ ] GHCRへイメージを公開し、Kubernetesに適用する
- [ ] Lit.linkのHTML構造変更を検出する定期Canaryを追加する
- [ ] ArgoCD Applicationを、独立リポジトリ公開後に有効化する
- [ ] `lumi-exporter` の名寄せアラートを実データで評価する（別repoのPoCを参照）
- [ ] `lumi-spec`の共通イベント・エンティティ契約を利用する
- [ ] `lumi-exporter`からentity endpointをpullして画面表示する

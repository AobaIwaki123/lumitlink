# ブラウザなしでのLit.link取得検証

## 結論

2026-10-12時点で、`curl`によるHTTP GETだけで `https://lit.link/amamiyasaku` のHTMLを取得できた。レスポンスはNext.jsのSSRページで、次の情報が含まれている。

- `title`、`description`、canonical、Open Graph、Twitter Cardのメタデータ
- プロフィール名、キャッチフレーズ、プロフィール本文、アイコンURL
- `profileLinks` として埋め込まれたリンク、画像、動画、音楽、ショップ、カレンダー情報

したがって、対象のAPIはブラウザ自動操作ではなく、HTTP取得とHTML/RSCデータ解析で実装できる。

## 実装上の判断

公開ページ取得にはGoの [`net/http`](https://pkg.go.dev/net/http) を使い、HTMLの外側は [`golang.org/x/net/html`](https://pkg.go.dev/golang.org/x/net/html) で解析する。Lit.link固有のSSRデータ解析は `internal/litlink` に閉じ込める。

一般的なリンク集抽出OSSとして、MITライセンスの [`ssut/biosites`](https://github.com/ssut/biosites) はLit.link対応を明記している。ただし今回必要なプロフィール本文、画像ブロック、動画、TimeTreeリンクまで保持するには、対象ページのSSRデータを直接正規化する方が情報を失いにくい。そのため初期実装ではブラウザ自動操作の依存を追加していない。

Lit.linkのサービス概要は [公式サイト](https://lit.link/) を参照する。今回の実装は公開プロフィールの読み取り専用であり、管理画面やログインAPIは扱わない。

## 再現コマンド

```bash
curl -L --compressed \
  -A 'Mozilla/5.0 (compatible; Lumipage/0.1)' \
  https://lit.link/amamiyasaku \
  -o /tmp/litlink-amamiyasaku.html
```

レスポンスの取得可否やHTML構造は変動し得るため、利用時はキャッシュ、タイムアウト、レスポンスサイズ上限、取得ログを有効にする。

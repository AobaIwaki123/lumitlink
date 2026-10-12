# lumi-exporter向けイベント名寄せメモ

## 方針

イベントを統合しても、入力ソースのイベントID、取得元URL、原文タイトルは削除しない。正規化後の `canonical_event_id` に `source_refs` をぶら下げ、判定理由を追跡できるようにする。

候補判定はタイトルの閾値を緩めに設定し、候補集合内の相対評価で順位を付ける。ただし、相対評価で一位が被った場合は成功扱いにしない。`ambiguous` 状態としてアラートを上げ、コードと重みを見直す対象にする。

## 判定レコード案

| フィールド | 内容 |
| :--- | :--- |
| `match_status` | `matched` / `candidate` / `ambiguous` / `unmatched` |
| `top_score` | 最高候補の総合スコア |
| `second_score` | 2位候補の総合スコア |
| `score_margin` | `top_score - second_score` |
| `feature_scores` | title/date/venue/artistごとの内訳 |
| `source_refs` | 元イベントID、ソース名、URL |
| `alert_reason` | 同率1位、マージン不足、根拠不足など |

### 同率1位の扱い

同率1位は「二つのイベントが同じ」ではなく、「現行の判定器が区別できない」と解釈する。次のようなアラートを一件として出す。

```json
{
  "type": "event_match_review",
  "reason": "top_score_tie",
  "top_score": 0.71,
  "candidates": [
    {"canonical_event_id": "event-a", "score": 0.71},
    {"canonical_event_id": "event-b", "score": 0.71}
  ],
  "source_refs": ["lumitree:event-123", "other:event-456"]
}
```

このアラートを見て、タイトル正規化の過剰な除去、日付のタイムゾーン、会場表記、出演者抽出の欠落を順番に確認する。アラートを握りつぶす閾値変更は行わない。

参考:

- [Unicode Standard Annex #15](https://unicode.org/reports/tr15/)
- [Fellegi and Sunter, A Theory for Record Linkage](https://doi.org/10.1080/01621459.1969.10501049)
- [Go `strings` package](https://pkg.go.dev/strings)

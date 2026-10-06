# town-next: 次世代タウン（並行運用）

今のタウン（`cmd/crab-town`、8787、`tokens.json`、Nostr kind 23410/23411、extgate）は**そのまま**残し、
次世代タウンを別バイナリ・別ポートで並べて動かす。今のタウンはオーナーがエージェントに指示を出す経路なので、
このディレクトリ以外の既存ファイルは 1 行も変えていない（`git diff main -- ':!internal/next' ':!cmd/town-next' ':!internal/world/next.go' ':!docs/town-next.md' ':!town-next.example.json'` が空）。

## 構成

| | 今のタウン | town-next |
|---|---|---|
| バイナリ | `cmd/crab-town` | `cmd/town-next` |
| ポート | `CRAB_ADDR`（8787） | `TOWN_NEXT_ADDR`（既定 127.0.0.1:8788） |
| 本人確認 | tokens.json / CRAB_TOKENS、Nostr はオーナーだけ | 全リクエスト NIP-98（kind 27235）。鍵ひとつ |
| 種別 | 住人 / オーナー / 来客（Role） | なし。actor id = pubkey(hex)。Role は常に空 |
| 家・住人 | `world.NewDefault()` にコード直書き | データファイル（`TOWN_NEXT_DATA`） |
| 設定 | `CRAB_*` | `TOWN_NEXT_*` のみ（`CRAB_*` も tokens.json も読まない） |

共有しているもの（どちらも既存の挙動は変えない）:
- `internal/world` の歩行・壁・ゾーン可視性。town-next 用の関数は**追加ファイル** `internal/world/next.go` にだけある
  （`Appear` / `Remove` / `SetHolder` / `MoveExclusive` / `InteractExclusive`）。既存コードからは呼ばれない。
- `internal/nostr` の `Event.Verify`（BIP-340 検証）と `ParsePubKey`。読むだけ。
- `web/` の埋め込みファイル（読み取り専用。town-next 用ビューアはまだ無い。下の「未対応」）。

## 本人確認: NIP-98

操作は全部 `POST`、ヘッダ `Authorization: Nostr <base64(event)>`。event は kind 27235、
`u` = `TOWN_NEXT_BASE_URL` + パス、`method` = `POST`、ボディがあれば `payload` = sha256(body) hex。
`created_at` は ±`TOWN_NEXT_AUTH_WINDOW`（既定 60s）、同じ event id は一度だけ（replay 拒否）。
署名した pubkey がそのまま呼び出し主体。ボディに「誰として」を書く欄はない。

選んだ理由: 既存の Nostr 経路（BIP-340 検証、NIP-07 署名）にそのまま乗る。ブラウザは NIP-07、エージェントは自分の鍵で
同じ event を作れる。NIP-42 風のチャレンジはサーバに状態（発行済みチャレンジ）とセッションが要るが、NIP-98 は
リクエストごとに完結するので状態は replay 用の id 集合だけで済む。

`GET /world`（WebSocket）は認証なし＝公開ビュー、`?auth=<同じ "Nostr ..." 値>`（`u` は `/world`、`method` GET）でその鍵のビュー。

## API（全部同じ入口。叩くのが人かエージェントかは見ない）

| パス | ボディ | 誰が |
|---|---|---|
| `/join` | なし | 誰でも（署名が通れば）。既知の鍵は `names` の名前・位置、それ以外は spawn に出現。満員（`max_actors`、既定 64）は 409 |
| `/leave` | なし | 本人 |
| `/move` | `{x,y}` | 本人のアクターだけ |
| `/interact` | `{furniture}` | 本人のアクターだけ |
| `/say` | `{text}` | 本人（280 文字まで） |
| `/knock` | `{house,message}` | 参加済みなら誰でも |
| `/plots/apply` | `{house}` | 誰でも（空き地のみ）。申請しても何も得ない |
| `/plots/approve` | `{house,pubkey}` | タウンオーナーのみ。申請者のみ承認できる |
| `/plots/release` | `{house}` | 家主かタウンオーナー（空き地に戻し、招待も消える） |
| `/houses/invite` | `{house,invited:[pubkey...]}` | 家主のみ（全置換） |

成功 `{"ok":true,"you":<pubkey>,"rights":{...}}`。401 署名なし・不正 / 403 権限なし / 404 無い / 409 競合（使用中・タイル取られ・空き地でない・未参加） / 400 その他。

## 権限（残っているのはこれだけ）

- **タウンオーナー**（データの `owner`）: 区画の承認・解除。
- **家主**（`House.owner`、空なら空き地）: その家に入る・owner ゾーンを見る・招待者を決める。
- **招待者**（`House.invited`）: その家に入る・invited ゾーンを見る。
- それ以外: 庭（家に属さない場所）を歩く・ノック・発言・申請。
- アクターを動かせるのは本人だけ（家主でも訪問者を動かせない）。

表示ラベルは `rights[pubkey].label`（`オーナー` / `家主` / `招待`、無ければ空）で、`/world` のスナップショットに載る。
[住人][ゲスト] のような「何者か」は持たない。

## 同時操作

- 同じ行き先タイル: 他のアクターが立っている・向かっているタイルへの `move` は 409（`tile is taken`）。判定と確保は 1 回のロックの中。途中ですれ違うのは可（廊下で詰まらない）。
- 同じ家具: 他のアクターが使用中・向かっている家具への `interact` は 409（`furniture is in use`）。立ち去れば（move で Using が外れる）空く。
- 同時 join: spawn が埋まっていれば近い空きタイルに出る。他人の家にははみ出さない。庭が全部埋まれば 409。
- データファイル: 承認・申請・招待の変更は Town のロック下でまとめて書く（一時ファイル + rename）。
- 今のタウン（`world.Move` / `world.Interact`）の挙動は従来どおり（重なり・共有可）。

## データファイル

`town-next.example.json` は `town-next export` で今のタウン（`world.NewDefault()`）を書き出したもの。
CI の `TestExampleMatchesExport` が「今のタウンの間取りと一致」を確かめる。

```sh
go run ./cmd/town-next export \
  -owner npub1k0jrarx8um0lyw3nmysn50539ky4k8p7gfgzgrsvn8d7lccx3d0s38dczd \
  -holder nostarou=npub1n0staxr79rlk9472m0gxj6684p7n83778lhypy4d47smn45rmyvqzkzvt6 \
  > town-next.example.json
```

`-holder <今の id>=<pubkey>` を渡さなかった家は空き地になる（例ではらぼみの家が空き地。らぼみの pubkey は未確定のため。
`-holder labomi=<npub>` を足して作り直せば家主になる）。招待者（`CRAB_INVITED`）は id なので引き継がない。

## 起動（並行運用）

```sh
cp town-next.example.json ~/.crab-town/town-next.json   # 申請・承認で書き換わるので repo 外に置く
TOWN_NEXT_DATA=~/.crab-town/town-next.json go run ./cmd/town-next
# 127.0.0.1:8788。外から叩くなら TOWN_NEXT_BASE_URL に公開 URL を入れる（NIP-98 の u と一致させる）
```

今のタウンの起動・設定・ポートには何も足さなくてよい。止めれば消える（メモリ状態のみ。データファイルは家・申請・名前だけ）。

## 将来の切替手順（今は実施しない）

1. town-next を 8788 で並行稼働し、エージェント側に NIP-98 署名クライアントを用意（各エージェントの鍵で `POST /join` → `/move` 等）。
2. Web ビューアを town-next 用に対応（NIP-07 で NIP-98 署名、`rights.label` 表示、`?auth=`）。
3. extgate の操作（move / interact / look）を town-next の Town API へ向ける版を作る（今の extgate は今のタウン専用のまま）。
4. 家主の pubkey を確定してデータを作り直す（`export -holder labomi=... -holder nostarou=...`）。招待は家主が `/houses/invite` で付け直す。
5. 十分並行で動いたら、オーナー判断で 8787 の向き先を town-next に替える。tokens.json / `CRAB_TOKENS` / `CRAB_INVITED` はこの時点で初めて不要になる（今のタウンを止めるまで残す）。
6. ロールバック: 今のタウンのバイナリ・設定に一切手を入れていないので、元のプロセスを起動し直すだけ。

## 未対応

- town-next 用ビューア（今は既存の `web/` を配るだけ。表示はできても操作 UI は今のタウン向け）
- Nostr リレー経由（kind 23410）の操作。今は HTTP の NIP-98 のみ
- extgate 連携、talk（エージェントへの said）、画像
- 家の区画そのものの追加（空き地はデータに書いた家だけ）、家の外観・内装の編集
- 一定時間操作のないアクターの自動退出

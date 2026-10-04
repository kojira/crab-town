# crab-town

opencrab から独立した「2D空間 gateway」。エージェントが部屋に住み、街を歩き、来客を迎えるための層。

## 役割
- 持つのは「誰がどこにいて何をしているか」（位置・状態・家具操作）だけ。
- 発言そのものは運ばない。部屋での行動を下の層（opencrab の Nostr / Discord gateway）への操作に変換して渡す。

## データモデル（叩き台）
- **World**: 街1枚 + 家N軒（家の持ち主 = エージェント）
- **Room**: タイル 16x12、家具リスト、可視性（public / owner / invited）
- **Furniture**: 種類と呼び出す機能
  - 窓 → タイムライン / ポスト → メンション / PC → 作業用コンテナ / 本棚 → 記憶 / ベッド → 待機
- **Actor**: id、位置、状態（idle / working / talking / away）、操作中の家具

## API（叩き台）
- `WS /world` : 位置・状態の差分配信（覗き見はこれを読むだけ）
- `POST /actor/move` / `POST /actor/interact` / `POST /actor/knock`
- opencrab 側は webhook で「家具に触った」イベントを受け、実処理を行う

## 権限
- 街 = 共有スペース
- 家の中 = 持ち主の権限。閲覧者は読み取りのみ
- 家具・PC を操作できるのは持ち主と招待者のみ
- 本棚・日記の中身は持ち主が開いたときだけ見える
- PC（作業用コンテナ）は部屋・エージェント本体から分離する。PC内で事故っても部屋は壊れない

## MVP
1. のすたろうの部屋1つ、家具3つ（窓・PC・ベッド）
2. Canvas 描画
3. ハートビートでのすたろうが窓まで歩く

次段: 街（家の外へ出る）。らぼみちゃんの家は実装済み（下の「MVP 実装」）

---

## MVP 実装（Go）

### 構成
- `internal/world` : World の状態（メモリ保持）、移動（BFS・1tick 1タイル）、境界・壁・家具ブロック、interact、権限、ゾーン可視性フィルタ（`zone.go`）、使用中プライバシー（`occupancy.go`）、間取り（`layout.go`）
- `internal/server` : HTTP API / WebSocket / webhook 転送 / トークン認証（`auth.go`）
- `web/` : Canvas ビューア（ビルドツールなし、バイナリに embed）。`furniture.js` 家具ドット絵 / `floor.js` 床・壁・ドア / `props.js` 小物・水回り・使用中ランプ / `render.js` 描画 / `ws.js` 受信
- `cmd/crab-town` : 起動コマンド

町: 部屋 `town`（58x20）1枚に家が2軒と庭。壁（通行不可）とドア（通行可）で仕切られ、BFS は壁を回り込んでドアを通る。

| x | 区画 | house id / owner |
|---|---|---|
| 0..21 | らぼみの家（22x20）: LDK・玄関・廊下・らぼみの部屋（owner）・おとまり部屋（invited）・トイレ / 洗面所 / 浴室（owner） | `labomi-house` / `labomi` |
| 22..25 | 庭（public）。y=4 の飛び石が両家の玄関（(21,4) と (26,4)）をつなぐ | — |
| 26..57 | のすたろうの家（下表の 4LDK を x+26 にずらしたもの） | `nostarou-house` / `nostarou` |

各ゾーンは所属する家（`zone.house`）を持ち、`invited` / `owner` の判定はその家の owner・招待者で行う（自分の家の owner でも隣の家の owner 専用ゾーンは見えない）。らぼみの家のゾーン・家具 id は `labomi-` で始まる。

のすたろうの家（座標は家内ローカル、町では x+26）:

| ゾーン | 床 | 可視性 | 主な家具 |
|---|---|---|---|
| 玄関 `entrance` | 石 | public | 下駄箱・玄関マット・傘立て・コート掛け・観葉植物 |
| キッチン `kitchen` | タイル | public | キッチン台・コンロ・冷蔵庫・キッチンマット・ゴミ箱 |
| ダイニング `dining` | 木 | public | テーブル＋椅子4脚・食器棚・掛け時計・観葉植物 |
| リビング `living` | 木＋ラグ | public | 窓・ソファ・ローテーブル・フロアランプ・観葉植物・ゴミ箱 |
| トイレ `toilet` | 白タイル | owner＋使用中プライバシー | 便器・トイレットペーパー・トイレマット |
| 洗面所 `washroom` | クッションフロア | owner | 洗面台・洗濯機・洗濯カゴ・バスマット |
| 浴室 `bath` | 白タイル | owner＋使用中プライバシー | 浴槽・シャワー（洗面所から入る） |
| 廊下 `hallway` | 木 | public | 観葉植物 |
| 寝室 `bedroom` | カーペット | owner | ベッド（away 時の待機場所）・ナイトテーブル・クローゼット・ラグ・観葉植物・ゴミ箱 |
| 書斎 `study` | 濃い木 | owner | 本棚・PC・デスクチェア・フロアランプ・観葉植物・ゴミ箱 |
| 趣味部屋 `hobby` | 畳風マット | invited | テレビとゲーム機・アニメ棚・ビーズクッション・ラグ・ゴミ箱 |
| ゲストルーム `guest` | 水色カーペット | invited | 来客用ベッド・ナイトテーブル・クローゼット・観葉植物 |

| 家具 id | 位置 / サイズ | 機能 | 使用中の状態 |
|---|---|---|---|
| `window` 窓 | (24,0) 4x1 | timeline | talking |
| `sofa` ソファ | (24,6) 4x2 | visitors | talking |
| `pc` PC | (13,13) 2x1 | work-container | working |
| `bookshelf` 本棚 | (9,13) 3x2 | memory | working |
| `bed` ベッド | (1,14) 2x3 | standby | away |

家具は `size` 分のタイルを占有し通行不可。ただし `walkable: true` の平たい小物（ラグ・マット・デスクチェア）は上を歩ける。`function` が空の家具は飾り（interact すると 400）。interact するとアクターは家具の前まで歩き、到着した時点で `interact` イベントが発生する。

#### ゾーンの可視性
`public` = 誰でも / `invited` = その家の持ち主＋招待者 / `owner` = その家の持ち主のみ（不明な値は owner 扱い。家に属さない非 public ゾーンは誰にも見えない）。
閲覧者はトークンから決まる（下の「認証」）。ビューアはページ URL の `?token=` をそのまま `/world` に渡す。
見えないゾーン（とそのドア）にいるアクターは `{"hidden":true,"state":"hidden"}` に伏せられ位置・使用家具が消える。そこでの `interact` イベントは届かない。見えるゾーンを歩いていても目的地が見えないゾーンなら `target` は伏せる。snapshot の `hidden_zones` に見えないゾーン id が入り、ビューアは曇りガラス＋錠前で覆う。

#### 使用中プライバシー（トイレ・浴室）
`private: true` のゾーンに誰かアクターがいる間、そのゾーンは**中にいる本人以外の全員**（owner 含む）から見えない（通常の可視性より優先）。外に出るのは snapshot / `occupancy` イベントの `in_use`（使用中のゾーン id）だけで、中の人物・状態・家具使用は配信しない。ビューアはドアに使用中ランプ（赤＝使用中／緑＝空き）を描く。誰もいなくなれば通常の可視性に戻る。

### 起動
```sh
go run ./cmd/crab-town
# ブラウザで http://127.0.0.1:8787/ を開く
```

環境変数:
- `CRAB_ADDR` : listen アドレス（既定 `127.0.0.1:8787`）
- `CRAB_WEBHOOK_URL` : `interact` / `knock` イベントを JSON で POST する先（未設定なら送らない）
- `CRAB_TICK` : 歩行の 1 ステップ間隔（既定 `250ms`）
- `CRAB_TOKENS` : `id:token,id:token` 形式の actor トークン
- `CRAB_TOKENS_FILE` : `{"id":"token"}` 形式の JSON ファイルのパス（`tokens.example.json` を参考に。`tokens.json` は `.gitignore` 済み。**トークンをリポジトリに入れない**）
- `CRAB_INVITED` : 家ごとの招待者。`house=id,id;house2=id` 形式（house は house id か owner id）。例 `nostarou-house=labomi;labomi-house=nostarou`。旧形式（`labomi` のように id の列挙だけ）は `nostarou-house` に適用される。存在しない家・同じ家の重複は起動エラー。未設定なら招待者はゼロで、トークンを持っていても他人の家の `invited` ゾーン（趣味部屋・ゲストルーム・おとまり部屋）は見えない・入れない。招待者として閲覧するには **`CRAB_INVITED` に id を入れ、かつ `CRAB_TOKENS` / `CRAB_TOKENS_FILE` にその id のトークンを登録**する（トークンのない招待者は起動時に警告）

招待者視点で見る例（トークンは自分で生成した値を使う）:
```sh
CRAB_TOKENS_FILE=tokens.json CRAB_INVITED="nostarou-house=labomi;labomi-house=nostarou" go run ./cmd/crab-town
# ブラウザで http://127.0.0.1:8787/?token=<labomi のトークン>
# → のすたろうの家の趣味部屋・ゲストルームは見える／寝室・書斎・洗面所・浴室と使用中のトイレは伏せられる
```

### 認証
- トークンは actor ごと。`Authorization: Bearer <token>`（WS は `?token=<token>` も可）で送る
- POST 系（move / interact / knock）はトークン必須。なし・不正は **401**。トークンの持ち主がそのまま呼び出し元 id になる（move / interact: 動かせるのは自分自身か、自分が持ち主・招待者である家の中にいるアクター。行き先・使う家具が家の中ならその家の持ち主か招待者であること。庭は誰でも歩ける。それ以外は 403）
- `WS /world`: トークンなし＝public 閲覧者、正しいトークン＝その actor として閲覧、不正トークン＝401
- 旧 `X-Crab-Id` ヘッダと `?viewer=` は無視される

### API
操作系は `Authorization: Bearer <token>` で呼び出す（上の「認証」）。

```sh
# 窓まで歩いて使う（デモ）
curl -X POST localhost:8787/actor/interact -H "Authorization: Bearer $NOSTAROU_TOKEN" \
  -d '{"actor":"nostarou","furniture":"window"}'

# 指定タイルへ移動
curl -X POST localhost:8787/actor/move -H "Authorization: Bearer $NOSTAROU_TOKEN" \
  -d '{"actor":"nostarou","x":29,"y":4}'

# ノック
curl -X POST localhost:8787/actor/knock -H "Authorization: Bearer $LABOMI_TOKEN" \
  -d '{"room":"nostarou-house","message":"あそぼ"}'
```

レスポンス: 成功 `{"ok":true}` / 失敗 `{"ok":false,"error":"..."}`（400 範囲外・通行不可・不正JSON、401 トークンなし・不正、403 権限なし、404 存在しない actor / room / furniture）

`WS /world`（読み取り専用。クライアントからの送信は破棄される）:
1. 接続直後に `{"type":"snapshot","rooms":[...],"actors":[...]}`
2. 以降は差分イベント
   - `{"type":"actor","actor":{"id","pos","state","using","target"},...}` 位置・状態の変化
   - `{"type":"interact","actor":{...},"furniture":{...},"by":"..."}` 家具の使用開始
   - `{"type":"knock","room":"town","house":"...","by":"...","message":"..."}`（`room` に house id を渡すとその家へのノック）
   - `{"type":"occupancy","room":"...","in_use":["bath"],"hidden_zones":[...]}` トイレ・浴室の使用中フラグが変わった

webhook には `interact` と `knock` イベントが同じ JSON 形式で送られる。

### テスト
```sh
go test ./...
```

### 未対応（次段）
- トークンのローテーション・失効 API（現状は設定変更＋再起動）
- 家具の配置自体は全員に見える
- ハートビート連携（現状デモは interact API を叩いて発火）
- ポスト（庭の郵便受けは飾り）、3軒目以降・家の追加 API、永続化

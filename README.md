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

次段: らぼみちゃんの家、街（家の外へ出る）

---

## MVP 実装（Go）

### 構成
- `internal/world` : World の状態（メモリ保持）、移動（BFS・1tick 1タイル）、境界・家具ブロック、interact、権限
- `internal/server` : HTTP API / WebSocket / webhook 転送
- `web/index.html` : Canvas ビューア（ビルドツールなし、バイナリに embed）
- `cmd/crab-town` : 起動コマンド

部屋: `nostarou-room`（16x12、owner = `nostarou`）

| 家具 id | 位置 | 機能 | 使用中の状態 |
|---|---|---|---|
| `window` 窓 | (7,0) | timeline | talking |
| `pc` PC | (14,5) | work-container | working |
| `bed` ベッド | (1,10) | standby | away |

家具タイルは通行不可。interact するとアクターは家具の前まで歩き、到着した時点で `interact` イベントが発生する。

### 起動
```sh
go run ./cmd/crab-town
# ブラウザで http://127.0.0.1:8787/ を開く
```

環境変数:
- `CRAB_ADDR` : listen アドレス（既定 `127.0.0.1:8787`）
- `CRAB_WEBHOOK_URL` : `interact` / `knock` イベントを JSON で POST する先（未設定なら送らない）
- `CRAB_TICK` : 歩行の 1 ステップ間隔（既定 `250ms`）

### API
操作系は `X-Crab-Id` ヘッダで呼び出し元を示す。move / interact は部屋の持ち主か招待者のみ（それ以外は 403）。knock は id があれば誰でも可。
**MVP では `X-Crab-Id` は識別のみで認証ではない。** 外部公開しないこと。

```sh
# 窓まで歩いて使う（デモ）
curl -X POST localhost:8787/actor/interact -H 'X-Crab-Id: nostarou' \
  -d '{"actor":"nostarou","furniture":"window"}'

# 指定タイルへ移動
curl -X POST localhost:8787/actor/move -H 'X-Crab-Id: nostarou' \
  -d '{"actor":"nostarou","x":3,"y":4}'

# ノック
curl -X POST localhost:8787/actor/knock -H 'X-Crab-Id: labomi' \
  -d '{"room":"nostarou-room","message":"あそぼ"}'
```

レスポンス: 成功 `{"ok":true}` / 失敗 `{"ok":false,"error":"..."}`（400 範囲外・通行不可・不正JSON、403 権限なし、404 存在しない actor / room / furniture）

`WS /world`（読み取り専用。クライアントからの送信は破棄される）:
1. 接続直後に `{"type":"snapshot","rooms":[...],"actors":[...]}`
2. 以降は差分イベント
   - `{"type":"actor","actor":{"id","pos","state","using","target"},...}` 位置・状態の変化
   - `{"type":"interact","actor":{...},"furniture":{...},"by":"..."}` 家具の使用開始
   - `{"type":"knock","room":"...","by":"...","message":"..."}`

webhook には `interact` と `knock` イベントが同じ JSON 形式で送られる。

### テスト
```sh
go test ./...
```

### 未対応（次段）
- 認証（`X-Crab-Id` は自己申告）
- 可視性 owner / invited の閲覧制限（現状 WS は全員に全状態を配信）
- ハートビート連携（現状デモは interact API を叩いて発火）
- 本棚・ポスト、複数の家、街、永続化

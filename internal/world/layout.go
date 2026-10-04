package world

// Default house: a 4LDK + water area on a 32x20 grid.
//
//	y=0        outer wall (window in the living room, front door on the left)
//	y=1..4     entrance | kitchen  dining | living   (LDK is open plan)
//	y=5        wall under kitchen / dining
//	y=6..8     entrance | toilet | washroom | bath | living
//	y=9        wall with doors to the hallway (bath is entered from the washroom)
//	y=10..11   hallway
//	y=12       wall with one door per private room
//	y=13..18   bedroom | study | hobby room | guest room
//	y=19       outer wall
const (
	HouseWidth  = 32
	HouseHeight = 20
)

// Zone visibilities.
const (
	VisPublic  = "public"  // anyone
	VisInvited = "invited" // owner + invited ids
	VisOwner   = "owner"   // owner only
)

func defaultZones() []*Zone {
	return []*Zone{
		{ID: "entrance", Name: "玄関", Floor: "stone", Visibility: VisPublic, Rect: Rect{1, 1, 4, 8}},
		{ID: "kitchen", Name: "キッチン", Floor: "tile", Visibility: VisPublic, Rect: Rect{6, 1, 7, 4}},
		{ID: "dining", Name: "ダイニング", Floor: "wood", Visibility: VisPublic, Rect: Rect{13, 1, 8, 4}},
		{ID: "living", Name: "リビング", Floor: "rug", Visibility: VisPublic, Rect: Rect{21, 1, 10, 8}},
		{ID: "toilet", Name: "トイレ", Floor: "bathtile", Visibility: VisOwner, Rect: Rect{6, 6, 3, 3}, Private: true},
		{ID: "washroom", Name: "洗面所", Floor: "cushionfloor", Visibility: VisOwner, Rect: Rect{10, 6, 5, 3}},
		{ID: "bath", Name: "浴室", Floor: "bathtile", Visibility: VisOwner, Rect: Rect{16, 6, 4, 3}, Private: true},
		{ID: "hallway", Name: "廊下", Floor: "hall", Visibility: VisPublic, Rect: Rect{1, 10, 30, 2}},
		{ID: "bedroom", Name: "寝室", Floor: "carpet", Visibility: VisOwner, Rect: Rect{1, 13, 7, 6}},
		{ID: "study", Name: "書斎", Floor: "darkwood", Visibility: VisOwner, Rect: Rect{9, 13, 7, 6}},
		{ID: "hobby", Name: "趣味部屋", Floor: "mat", Visibility: VisInvited, Rect: Rect{17, 13, 7, 6}},
		{ID: "guest", Name: "ゲストルーム", Floor: "bluecarpet", Visibility: VisInvited, Rect: Rect{25, 13, 6, 6}},
	}
}

func defaultWalls() []Rect {
	return []Rect{
		{0, 0, HouseWidth, 1}, {0, HouseHeight - 1, HouseWidth, 1}, // outer top / bottom
		{0, 0, 1, HouseHeight}, {HouseWidth - 1, 0, 1, HouseHeight}, // outer left / right
		{5, 1, 1, 8},                               // entrance | LDK / water area
		{6, 5, 15, 1},                              // kitchen + dining | water area
		{9, 6, 1, 3}, {15, 6, 1, 3}, {20, 6, 1, 3}, // toilet | washroom | bath | living
		{0, 9, HouseWidth, 1},                         // upper floor | hallway
		{0, 12, HouseWidth, 1},                        // hallway | private rooms
		{8, 13, 1, 6}, {16, 13, 1, 6}, {24, 13, 1, 6}, // between private rooms
	}
}

func defaultDoors() []Pos {
	return []Pos{
		{0, 4},          // front door
		{5, 3},          // entrance -> LDK
		{3, 9}, {23, 9}, // entrance / living -> hallway
		{7, 9}, {12, 9}, {15, 7}, // hallway -> toilet / washroom, washroom -> bath
		{4, 12}, {12, 12}, {20, 12}, {28, 12}, // hallway -> bedroom / study / hobby / guest
	}
}

func defaultFurniture() []*Furniture {
	deco := func(id, kind, label string, x, y, w, h int) *Furniture {
		return &Furniture{ID: id, Kind: kind, Label: label, Pos: Pos{x, y}, Size: Size{w, h}}
	}
	flat := func(id, kind, label string, x, y, w, h int) *Furniture { // rugs, mats, desk chair
		f := deco(id, kind, label, x, y, w, h)
		f.Walkable = true
		return f
	}
	return []*Furniture{
		// entrance
		deco("shoebox", KindShoebox, "下駄箱", 1, 1, 1, 3),
		flat("doormat", KindDoormat, "玄関マット", 1, 4, 1, 1),
		deco("umbrella", KindUmbrella, "傘立て", 4, 1, 1, 1),
		deco("coatrack", KindCoatRack, "コート掛け", 1, 7, 1, 1),
		deco("entplant", KindPlant, "観葉植物", 4, 8, 1, 1),
		// kitchen (decoration only)
		deco("counter", KindCounter, "キッチン台", 6, 1, 4, 1),
		deco("stove", KindStove, "コンロ", 10, 1, 2, 1),
		deco("fridge", KindFridge, "冷蔵庫", 12, 1, 1, 2),
		flat("kitchenmat", KindKitchenMat, "キッチンマット", 6, 2, 4, 1),
		deco("kitchentrash", KindTrash, "ゴミ箱", 6, 4, 1, 1),
		// dining
		deco("table", KindTable, "ダイニングテーブル", 15, 2, 3, 2),
		deco("chair1", KindChair, "椅子", 14, 2, 1, 1),
		deco("chair2", KindChair, "椅子", 14, 3, 1, 1),
		deco("chair3", KindChair, "椅子", 18, 2, 1, 1),
		deco("chair4", KindChair, "椅子", 18, 3, 1, 1),
		deco("clock", KindClock, "掛け時計", 16, 0, 1, 1), // wall-mounted
		deco("sideboard", KindSideboard, "食器棚", 19, 1, 2, 1),
		deco("diningplant", KindPlant, "観葉植物", 20, 4, 1, 1),
		// living
		{ID: "window", Kind: KindWindow, Label: "窓", Function: "timeline", Pos: Pos{24, 0}, Size: Size{4, 1}, Access: Pos{25, 1}, State: StateTalking},
		deco("lowtable", KindLowTable, "ローテーブル", 25, 3, 2, 1),
		{ID: "sofa", Kind: KindSofa, Label: "ソファ", Function: "visitors", Pos: Pos{24, 6}, Size: Size{4, 2}, Access: Pos{26, 5}, State: StateTalking},
		deco("plant", KindPlant, "観葉植物", 30, 1, 1, 1),
		deco("livingplant", KindPlant, "観葉植物", 21, 1, 1, 1),
		deco("floorlamp", KindFloorLamp, "フロアランプ", 30, 7, 1, 1),
		deco("livingtrash", KindTrash, "ゴミ箱", 21, 8, 1, 1),
		// toilet
		deco("toiletbowl", KindToilet, "便器", 7, 6, 1, 1),
		deco("paper", KindPaper, "トイレットペーパー", 8, 6, 1, 1),
		flat("toiletmat", KindToiletMat, "トイレマット", 7, 7, 1, 1),
		// washroom
		deco("vanity", KindVanity, "洗面台", 10, 6, 2, 1),
		deco("washer", KindWasher, "洗濯機", 14, 6, 1, 1),
		deco("laundry", KindLaundry, "洗濯カゴ", 10, 8, 1, 1),
		// bath
		deco("shower", KindShower, "シャワー", 16, 6, 1, 1),
		deco("bathtub", KindBathtub, "浴槽", 18, 6, 2, 3),
		flat("bathmat", KindBathMat, "バスマット", 14, 7, 1, 1),
		// hallway
		deco("hallplant", KindPlant, "観葉植物", 30, 10, 1, 1),
		// bedroom
		{ID: "bed", Kind: KindBed, Label: "ベッド", Function: "standby", Pos: Pos{1, 14}, Size: Size{2, 3}, Access: Pos{3, 15}, State: StateAway},
		deco("nightstand", KindNightstand, "ナイトテーブル", 3, 14, 1, 1),
		deco("wardrobe", KindWardrobe, "クローゼット", 6, 13, 2, 1),
		flat("bedrug", KindRug, "ラグ", 4, 16, 3, 2),
		deco("bedplant", KindPlant, "観葉植物", 1, 18, 1, 1),
		deco("bedtrash", KindTrash, "ゴミ箱", 7, 18, 1, 1),
		// study
		{ID: "bookshelf", Kind: KindBookshelf, Label: "本棚", Function: "memory", Pos: Pos{9, 13}, Size: Size{3, 2}, Access: Pos{10, 15}, State: StateWorking},
		{ID: "pc", Kind: KindPC, Label: "PC", Function: "work-container", Pos: Pos{13, 13}, Size: Size{2, 1}, Access: Pos{13, 14}, State: StateWorking},
		flat("deskchair", KindDeskChair, "デスクチェア", 13, 14, 1, 1),
		deco("studytrash", KindTrash, "ゴミ箱", 15, 13, 1, 1),
		deco("studylamp", KindFloorLamp, "フロアランプ", 15, 18, 1, 1),
		deco("studyplant", KindPlant, "観葉植物", 9, 18, 1, 1),
		// hobby room
		deco("tv", KindTV, "テレビとゲーム機", 17, 13, 3, 1),
		deco("figureshelf", KindFigureShelf, "アニメ棚", 22, 13, 2, 2),
		deco("beanbag", KindBeanbag, "ビーズクッション", 18, 15, 1, 1),
		flat("hobbyrug", KindRug, "ラグ", 17, 16, 3, 2),
		deco("hobbytrash", KindTrash, "ゴミ箱", 23, 18, 1, 1),
		// guest room
		deco("guestbed", KindGuestBed, "来客用ベッド", 25, 14, 2, 3),
		deco("guestplant", KindPlant, "観葉植物", 30, 13, 1, 1),
		deco("guestnight", KindNightstand, "ナイトテーブル", 27, 14, 1, 1),
		deco("guestcloset", KindWardrobe, "クローゼット", 27, 18, 2, 1),
	}
}

// NewDefault builds the default world: nostarou's 4LDK house.
func NewDefault() *World {
	w := New()
	w.AddRoom(&Room{
		ID: "nostarou-room", Owner: "nostarou", Visibility: VisPublic,
		Width: HouseWidth, Height: HouseHeight,
		Zones: defaultZones(), Walls: defaultWalls(), Doors: defaultDoors(),
		Furniture: defaultFurniture(),
	})
	w.AddActor(&Actor{ID: "nostarou", Name: "のすたろう", RoomID: "nostarou-room", Pos: Pos{22, 3}, State: StateIdle})
	return w
}

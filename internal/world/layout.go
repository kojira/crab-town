package world

// Default house: a 4LDK on a 32x20 grid.
//
//	y=0        outer wall (window in the living room, front door on the left)
//	y=1..8     entrance | kitchen  dining  living   (LDK is open plan)
//	y=9        wall with doors to the hallway
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
		{ID: "kitchen", Name: "キッチン", Floor: "tile", Visibility: VisPublic, Rect: Rect{6, 1, 7, 8}},
		{ID: "dining", Name: "ダイニング", Floor: "wood", Visibility: VisPublic, Rect: Rect{13, 1, 8, 8}},
		{ID: "living", Name: "リビング", Floor: "rug", Visibility: VisPublic, Rect: Rect{21, 1, 10, 8}},
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
		{5, 1, 1, 8},                                  // entrance | LDK
		{0, 9, HouseWidth, 1},                         // LDK | hallway
		{0, 12, HouseWidth, 1},                        // hallway | private rooms
		{8, 13, 1, 6}, {16, 13, 1, 6}, {24, 13, 1, 6}, // between private rooms
	}
}

func defaultDoors() []Pos {
	return []Pos{
		{0, 4},          // front door
		{5, 5},          // entrance -> LDK
		{3, 9}, {14, 9}, // entrance / LDK -> hallway
		{4, 12}, {12, 12}, {20, 12}, {28, 12}, // hallway -> bedroom / study / hobby / guest
	}
}

func defaultFurniture() []*Furniture {
	deco := func(id, kind, label string, x, y, w, h int) *Furniture {
		return &Furniture{ID: id, Kind: kind, Label: label, Pos: Pos{x, y}, Size: Size{w, h}}
	}
	return []*Furniture{
		// entrance
		deco("shoebox", KindShoebox, "下駄箱", 1, 1, 1, 3),
		// kitchen (decoration only)
		deco("counter", KindCounter, "キッチン台", 6, 1, 4, 1),
		deco("stove", KindStove, "コンロ", 10, 1, 2, 1),
		deco("fridge", KindFridge, "冷蔵庫", 12, 1, 1, 2),
		// dining
		deco("table", KindTable, "ダイニングテーブル", 15, 3, 3, 2),
		deco("chair1", KindChair, "椅子", 15, 2, 1, 1),
		deco("chair2", KindChair, "椅子", 17, 2, 1, 1),
		deco("chair3", KindChair, "椅子", 15, 5, 1, 1),
		deco("chair4", KindChair, "椅子", 17, 5, 1, 1),
		// living
		{ID: "window", Kind: KindWindow, Label: "窓", Function: "timeline", Pos: Pos{24, 0}, Size: Size{4, 1}, Access: Pos{25, 1}, State: StateTalking},
		deco("lowtable", KindLowTable, "ローテーブル", 25, 3, 2, 1),
		{ID: "sofa", Kind: KindSofa, Label: "ソファ", Function: "visitors", Pos: Pos{24, 6}, Size: Size{4, 2}, Access: Pos{26, 5}, State: StateTalking},
		deco("plant", KindPlant, "観葉植物", 30, 1, 1, 1),
		// bedroom
		{ID: "bed", Kind: KindBed, Label: "ベッド", Function: "standby", Pos: Pos{1, 14}, Size: Size{2, 3}, Access: Pos{3, 15}, State: StateAway},
		deco("nightstand", KindNightstand, "ナイトテーブル", 3, 14, 1, 1),
		deco("wardrobe", KindWardrobe, "クローゼット", 6, 13, 2, 1),
		// study
		{ID: "bookshelf", Kind: KindBookshelf, Label: "本棚", Function: "memory", Pos: Pos{9, 13}, Size: Size{3, 2}, Access: Pos{10, 15}, State: StateWorking},
		{ID: "pc", Kind: KindPC, Label: "PC", Function: "work-container", Pos: Pos{13, 13}, Size: Size{2, 1}, Access: Pos{13, 14}, State: StateWorking},
		// hobby room
		deco("tv", KindTV, "テレビとゲーム機", 17, 13, 3, 1),
		deco("figureshelf", KindFigureShelf, "アニメ棚", 22, 13, 2, 2),
		deco("beanbag", KindBeanbag, "ビーズクッション", 18, 15, 1, 1),
		// guest room
		deco("guestbed", KindGuestBed, "来客用ベッド", 25, 14, 2, 3),
		deco("guestplant", KindPlant, "観葉植物", 30, 13, 1, 1),
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

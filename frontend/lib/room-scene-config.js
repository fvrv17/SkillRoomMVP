export const ROOM_LEVELS = ["bronze", "silver", "gold", "platinum"];

export const ROOM_THEMES = {
  classic: {
    id: "classic",
    label: "Classic shell",
    description: "Clean studio box for the default SkillRoom experience.",
    shellAsset: "/room/shell/classic-shell.svg",
    overlayAsset: "",
  },
  holiday: {
    id: "holiday",
    label: "Holiday shell",
    description: "Seasonal overlay layer for limited-time drops and monetized decor packs.",
    shellAsset: "/room/shell/classic-shell.svg",
    overlayAsset: "/room/themes/holiday-overlay.svg",
  },
};

export const ROOM_WINDOW_SCENES = {
  daylight: {
    id: "daylight",
    label: "Daylight",
    asset: "/room/windows/daylight.svg",
  },
  sunset: {
    id: "sunset",
    label: "Sunset",
    asset: "/room/windows/sunset.svg",
  },
  night: {
    id: "night",
    label: "Night",
    asset: "/room/windows/night.svg",
  },
};

export const ROOM_DEFAULT_THEME_ID = "classic";
export const ROOM_DEFAULT_WINDOW_SCENE_ID = "daylight";
export const ROOM_DEFAULT_WALL_STYLE = "wall_cream_default";
export const ROOM_DEFAULT_FLOOR_STYLE = "floor_oak_default";

function buildItemAssets(code) {
  return Object.fromEntries(ROOM_LEVELS.map((level) => [level, roomAssetPath(code, level)]));
}

export const ROOM_WALL_STYLES = {
  wall_cream_default: {
    code: "wall_cream_default",
    label: "Studio Cream",
    preview: "Warm neutral",
    premium: false,
    className: "room-stage__wall-tint--cream",
  },
  wall_sand_default: {
    code: "wall_sand_default",
    label: "Soft Sand",
    preview: "Muted sand",
    premium: false,
    className: "room-stage__wall-tint--sand",
  },
  wall_graphite_plus: {
    code: "wall_graphite_plus",
    label: "Graphite",
    preview: "Premium dark",
    premium: true,
    className: "room-stage__wall-tint--graphite",
  },
};

export const ROOM_FLOOR_STYLES = {
  floor_oak_default: {
    code: "floor_oak_default",
    label: "Light Oak",
    preview: "Base floor",
    premium: false,
    className: "room-stage__floor-tint--oak",
  },
  floor_honey_default: {
    code: "floor_honey_default",
    label: "Honey Oak",
    preview: "Warm oak",
    premium: false,
    className: "room-stage__floor-tint--honey",
  },
  floor_charcoal_plus: {
    code: "floor_charcoal_plus",
    label: "Charcoal",
    preview: "Premium dark",
    premium: true,
    className: "room-stage__floor-tint--charcoal",
  },
};

export const ROOM_DECOR_SLOTS = {
  decor_left: {
    slotCode: "decor_left",
    label: "Left decor",
    style: {
      left: "24.4%",
      bottom: "33%",
      width: "9.6%",
      height: "13.4%",
      zIndex: 15,
    },
  },
  decor_right: {
    slotCode: "decor_right",
    label: "Right decor",
    style: {
      right: "13.4%",
      top: "19.8%",
      width: "8.8%",
      height: "17%",
      zIndex: 16,
    },
  },
  decor_wall: {
    slotCode: "decor_wall",
    label: "Wall decor",
    style: {
      left: "12.8%",
      top: "16.8%",
      width: "10.4%",
      height: "25%",
      zIndex: 14,
    },
  },
};

export const ROOM_DECOR_ITEMS = {
  decor_books_orange: {
    code: "decor_books_orange",
    slotCode: "decor_left",
    label: "Orange Book Stack",
    premium: true,
    className: "room-decor--books-orange",
  },
  decor_lamp_black: {
    code: "decor_lamp_black",
    slotCode: "decor_right",
    label: "Black Studio Lamp",
    premium: true,
    className: "room-decor--lamp-black",
  },
  decor_poster_grid: {
    code: "decor_poster_grid",
    slotCode: "decor_wall",
    label: "Grid Poster",
    premium: true,
    className: "room-decor--poster-grid",
  },
};

export const ROOM_SCENE_ORDER = ["shelf", "trophy_case", "monitor", "desk", "chair", "plant"];

export const ROOM_SCENE_SLOTS = {
  shelf: {
    code: "shelf",
    title: "Shelf / Volume",
    skill: "Volume",
    placeholderTitle: "Shelf placeholder",
    availableLevels: [],
    assets: buildItemAssets("shelf"),
    style: {
      left: "72.2%",
      top: "18.2%",
      width: "17.2%",
      height: "12.8%",
      zIndex: 22,
      transform: "translate3d(0, 0, 0)",
    },
  },
  trophy_case: {
    code: "trophy_case",
    title: "Trophy / Achievements",
    skill: "Achievements",
    placeholderTitle: "Trophy placeholder",
    presentationMode: "achievement_case",
    showLevelBadge: false,
    availableLevels: [],
    assets: buildItemAssets("trophy_case"),
    style: {
      left: "55.8%",
      top: "23.4%",
      width: "12.4%",
      height: "20.8%",
      zIndex: 18,
      transform: "translate3d(0, 0, 0)",
    },
  },
  monitor: {
    code: "monitor",
    title: "Monitor / React",
    skill: "React",
    placeholderTitle: "Monitor placeholder",
    availableLevels: [],
    assets: buildItemAssets("monitor"),
    style: {
      left: "57.8%",
      bottom: "34.5%",
      width: "15.8%",
      height: "13.8%",
      zIndex: 28,
      transform: "translate3d(-50%, 0, 0)",
    },
  },
  desk: {
    code: "desk",
    title: "Desk / JavaScript",
    skill: "JavaScript",
    placeholderTitle: "Desk placeholder",
    availableLevels: [],
    assets: buildItemAssets("desk"),
    style: {
      left: "65.6%",
      bottom: "21%",
      width: "35%",
      height: "17.4%",
      zIndex: 17,
      transform: "translate3d(-50%, 0, 0)",
    },
  },
  chair: {
    code: "chair",
    title: "Chair / Architecture",
    skill: "Architecture",
    placeholderTitle: "Chair placeholder",
    availableLevels: [],
    assets: buildItemAssets("chair"),
    style: {
      left: "41.8%",
      bottom: "19.5%",
      width: "17.4%",
      height: "24.8%",
      zIndex: 24,
      transform: "translate3d(-50%, 0, 0)",
    },
  },
  plant: {
    code: "plant",
    title: "Plant / Consistency",
    skill: "Consistency",
    placeholderTitle: "Plant placeholder",
    availableLevels: [],
    assets: buildItemAssets("plant"),
    style: {
      right: "9.6%",
      bottom: "21.5%",
      width: "10.6%",
      height: "20.8%",
      zIndex: 24,
      transform: "translate3d(0, 0, 0)",
    },
  },
};

export function normalizeRoomLevel(level) {
  const normalized = String(level || "").toLowerCase();
  return ROOM_LEVELS.includes(normalized) ? normalized : "bronze";
}

export function normalizeRoomTheme(themeID) {
  return ROOM_THEMES[themeID] ? themeID : ROOM_DEFAULT_THEME_ID;
}

export function normalizeWindowScene(sceneID) {
  return ROOM_WINDOW_SCENES[sceneID] ? sceneID : ROOM_DEFAULT_WINDOW_SCENE_ID;
}

export function resolveRoomCustomization(equipped = []) {
  const equippedMap = Object.fromEntries((equipped || []).map((item) => [item.slot_code, item.cosmetic_code]));
  const windowCode = equippedMap.window_scene || "window_daylight_default";
  const wallCode = ROOM_WALL_STYLES[equippedMap.wall_style] ? equippedMap.wall_style : ROOM_DEFAULT_WALL_STYLE;
  const floorCode = ROOM_FLOOR_STYLES[equippedMap.floor_style] ? equippedMap.floor_style : ROOM_DEFAULT_FLOOR_STYLE;
  const decor = Object.entries(ROOM_DECOR_SLOTS)
    .map(([slotCode, slot]) => {
      const cosmeticCode = equippedMap[slotCode];
      if (!cosmeticCode || !ROOM_DECOR_ITEMS[cosmeticCode]) {
        return null;
      }
      return {
        slotCode,
        ...slot,
        ...ROOM_DECOR_ITEMS[cosmeticCode],
      };
    })
    .filter(Boolean);

  return {
    windowSceneID: windowSceneForCosmetic(windowCode),
    wallStyle: ROOM_WALL_STYLES[wallCode],
    floorStyle: ROOM_FLOOR_STYLES[floorCode],
    decor,
  };
}

function windowSceneForCosmetic(cosmeticCode) {
  switch (cosmeticCode) {
    case "window_sunset_default":
      return "sunset";
    case "window_night_plus":
      return "night";
    default:
      return ROOM_DEFAULT_WINDOW_SCENE_ID;
  }
}

export function roomAssetPath(code, level) {
  return `/room-items/${code}/${normalizeRoomLevel(level)}.png`;
}

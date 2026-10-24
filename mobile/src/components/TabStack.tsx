import { Platform } from "react-native";
import { Stack } from "expo-router";
import { useTheme } from "../lib/theme";
import { useStackOptions } from "./Header";

/**
 * The navigation stack inside each tab: a native header with a collapsing large title on iOS.
 * On the web preview the header is hidden and the screen draws the large title itself (see Screen `webTitle`).
 */
export default function TabStack() {
  const { c } = useTheme();
  const base = useStackOptions();
  return (
    <Stack
      screenOptions={{
        ...base,
        headerShown: Platform.OS !== "web",
        headerLargeTitle: Platform.OS === "ios",
        headerLargeTitleShadowVisible: false,
        headerLargeStyle: { backgroundColor: c.bg as string },
        headerBackButtonDisplayMode: "minimal",
      }}
    />
  );
}

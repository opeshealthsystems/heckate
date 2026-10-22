import { useEffect } from "react";
import { Cinzel_700Bold } from "@expo-google-fonts/cinzel";
import { IBMPlexMono_500Medium, IBMPlexMono_600SemiBold } from "@expo-google-fonts/ibm-plex-mono";
import { IBMPlexSans_400Regular, IBMPlexSans_500Medium, IBMPlexSans_600SemiBold } from "@expo-google-fonts/ibm-plex-sans";
import { useFonts } from "expo-font";
import { Stack } from "expo-router";
import * as SplashScreen from "expo-splash-screen";
import { StatusBar } from "expo-status-bar";
import { View } from "react-native";
import { SafeAreaProvider } from "react-native-safe-area-context";
import { LockGate } from "../components/LockScreen";
import { LockProvider } from "../lib/lock";
import { SessionProvider } from "../lib/session";
import { ThemeProvider, useTheme } from "../lib/theme";

// Keep the splash screen up until the brand fonts are ready (or have failed: the app still works with system fonts).
void SplashScreen.preventAutoHideAsync();

function Shell() {
  const { c } = useTheme();
  return (
    <View style={{ flex: 1, backgroundColor: c.bg }}>
      <StatusBar style={c.mode === "dark" ? "light" : "dark"} />
      <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: c.bg }, animation: "slide_from_right" }} />
      <LockGate />
    </View>
  );
}

export default function RootLayout() {
  const [loaded, error] = useFonts({ Cinzel_700Bold, IBMPlexSans_400Regular, IBMPlexSans_500Medium, IBMPlexSans_600SemiBold, IBMPlexMono_500Medium, IBMPlexMono_600SemiBold });
  useEffect(() => {
    if (loaded || error) void SplashScreen.hideAsync();
  }, [loaded, error]);
  if (!loaded && !error) return null;
  return (
    <SafeAreaProvider>
      <ThemeProvider>
        <SessionProvider>
          <LockProvider>
            <Shell />
          </LockProvider>
        </SessionProvider>
      </ThemeProvider>
    </SafeAreaProvider>
  );
}

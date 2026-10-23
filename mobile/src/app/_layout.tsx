import { useEffect } from "react";
import { Cinzel_700Bold } from "@expo-google-fonts/cinzel";
import { useFonts } from "expo-font";
import { Stack } from "expo-router";
import * as SplashScreen from "expo-splash-screen";
import { StatusBar } from "expo-status-bar";
import { View } from "react-native";
import { SafeAreaProvider } from "react-native-safe-area-context";
import { useStackOptions } from "../components/Header";
import { LockGate } from "../components/LockScreen";
import { LockProvider } from "../lib/lock";
import { SessionProvider } from "../lib/session";
import { ThemeProvider, useTheme } from "../lib/theme";

// Keep the splash screen up until the wordmark font is ready (or has failed: the wordmark then falls back to the system font).
void SplashScreen.preventAutoHideAsync();

function Shell() {
  const { c } = useTheme();
  const base = useStackOptions();
  return (
    <View style={{ flex: 1, backgroundColor: c.canvas as string }}>
      <StatusBar style="auto" />
      <Stack screenOptions={{ ...base, headerShown: false, headerBackButtonDisplayMode: "minimal" }}>
        <Stack.Screen name="ticket/[id]" options={{ headerShown: true, title: "Ticket" }} />
        <Stack.Screen name="intervention/[id]" options={{ headerShown: true, title: "Site visit" }} />
        <Stack.Screen name="document/[ref]" options={{ headerShown: true, title: "Document" }} />
        <Stack.Screen name="report" options={{ presentation: "modal", headerShown: true }} />
      </Stack>
      <LockGate />
    </View>
  );
}

export default function RootLayout() {
  // Cinzel is loaded for the HEKATE wordmark on the sign-in and lock screens only. Everything else is the system font.
  const [loaded, error] = useFonts({ Cinzel_700Bold });
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

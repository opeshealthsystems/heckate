import { useEffect } from "react";
import { Cinzel_700Bold } from "@expo-google-fonts/cinzel";
import { IBMPlexSans_400Regular, IBMPlexSans_500Medium, IBMPlexSans_600SemiBold } from "@expo-google-fonts/ibm-plex-sans";
import { useFonts } from "expo-font";
import { Stack } from "expo-router";
import * as SplashScreen from "expo-splash-screen";
import { StatusBar } from "expo-status-bar";
import { SessionProvider } from "../lib/session";
import { font, useTheme } from "../lib/theme";

// Keep the splash screen up until the brand fonts are ready (or have failed: the app still works with system fonts).
void SplashScreen.preventAutoHideAsync();

export default function RootLayout() {
  const { c } = useTheme();
  const [loaded, error] = useFonts({ Cinzel_700Bold, IBMPlexSans_400Regular, IBMPlexSans_500Medium, IBMPlexSans_600SemiBold });
  useEffect(() => {
    if (loaded || error) void SplashScreen.hideAsync();
  }, [loaded, error]);
  if (!loaded && !error) return null;
  const header = { headerStyle: { backgroundColor: c.brand }, headerTintColor: "#ffffff", headerTitleStyle: { fontFamily: font.semibold } };
  return (
    <SessionProvider>
      <StatusBar style="light" />
      <Stack screenOptions={{ headerShown: false }}>
        <Stack.Screen name="ticket/[id]" options={{ headerShown: true, title: "Ticket", ...header }} />
        <Stack.Screen name="intervention/[id]" options={{ headerShown: true, title: "Site visit", ...header }} />
      </Stack>
    </SessionProvider>
  );
}

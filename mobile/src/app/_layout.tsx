import { Stack } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { SessionProvider } from "../lib/session";

export default function RootLayout() {
  return (
    <SessionProvider>
      <StatusBar style="auto" />
      <Stack screenOptions={{ headerShown: false }}>
        <Stack.Screen name="ticket/[id]" options={{ headerShown: true, title: "Ticket" }} />
        <Stack.Screen name="intervention/[id]" options={{ headerShown: true, title: "Site visit" }} />
      </Stack>
    </SessionProvider>
  );
}

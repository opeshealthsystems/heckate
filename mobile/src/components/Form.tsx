import type { ReactNode } from "react";
import { View } from "react-native";
import { Stack } from "expo-router";
import { sp } from "../lib/theme";
import type { Outcome } from "../lib/self-service";
import { Banner } from "./Banner";
import { Button } from "./Button";
import { Screen } from "./Screen";

/** A pushed form page: plain header with a back button, grouped fields, the outcome as a banner, one primary button. */
export function FormPage({ title, out, submitTitle, disabled, onSubmit, children }: { title: string; out: Outcome | null; submitTitle: string; disabled?: boolean; onSubmit: () => void; children: ReactNode }) {
  return (
    <Screen keyboard>
      <Stack.Screen options={{ title, headerShown: true, headerLargeTitle: false }} />
      {children}
      {out && <Banner icon={out.ok ? "checkCircle" : "warning"} tone={out.ok ? "label2" : "red"}>{out.text}</Banner>}
      <View style={{ paddingBottom: sp.sm }}>
        <Button kind="primary" title={submitTitle} disabled={disabled} onPress={onSubmit} />
      </View>
    </Screen>
  );
}

import * as ImagePicker from "expo-image-picker";

export interface Photo { uri: string; name: string; type: string }

/** Takes or picks one photo. Returns null if the user cancels; throws a message they can act on. */
export async function pickPhoto(source: "camera" | "library"): Promise<Photo | null> {
  if (source === "camera") {
    const perm = await ImagePicker.requestCameraPermissionsAsync();
    if (!perm.granted) throw new Error("Allow camera access to take a photo.");
  }
  const options: ImagePicker.ImagePickerOptions = { mediaTypes: ["images"], quality: 0.7 }; // JPEG at 70%: small enough for mobile data
  const result = source === "camera" ? await ImagePicker.launchCameraAsync(options) : await ImagePicker.launchImageLibraryAsync(options);
  if (result.canceled || !result.assets?.length) return null;
  const a = result.assets[0];
  const type = a.mimeType && /^image\/(jpeg|png|webp)$/.test(a.mimeType) ? a.mimeType : "image/jpeg";
  const name = a.fileName ?? `photo-${Date.now()}.${type === "image/png" ? "png" : type === "image/webp" ? "webp" : "jpg"}`;
  return { uri: a.uri, name, type };
}

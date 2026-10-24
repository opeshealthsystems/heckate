import * as Location from "expo-location";

export interface Fix { lat: number; lng: number }

/** Current position, or throws a message the engineer can act on. */
export async function currentFix(): Promise<Fix> {
  const perm = await Location.requestForegroundPermissionsAsync();
  if (!perm.granted) throw new Error("Allow location access to check in on site.");
  const pos = await Location.getCurrentPositionAsync({ accuracy: Location.Accuracy.High });
  return { lat: pos.coords.latitude, lng: pos.coords.longitude };
}

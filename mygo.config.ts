export default {
  name: "Rainforest",
  id: "com.thedavidweng.rainforest",
  identifier: "com.thedavidweng.rainforest",
  version: "0.1.0",
  copyright: "Copyright (C) 2026 Davy Weng and contributors",
  main: ".",
  i18n: { dir: "locales", source: "en" },
  updates: {
    github: "thedavidweng/rainforest",
    tagPrefix: "v",
    publicKey:
      process.env.MYGO_UPDATER_PUBLIC_KEY ||
      "Ykj6GObFAitTNBjip51D96NsOrUx1aUbP9j5yy+cJ8s=",
    changelog: "CHANGELOG.md",
  },
  macos: {
    minimumSystemVersion: "12.0",
    dmgTitle: "Rainforest",
    ...(process.env.MACOS_SIGNING_IDENTITY
      ? { signingIdentity: process.env.MACOS_SIGNING_IDENTITY }
      : {}),
    ...(process.env.MACOS_NOTARIZE_PROFILE
      ? { notarize: {
          keychainProfile: process.env.MACOS_NOTARIZE_PROFILE,
          keychain: process.env.APPLE_SIGNING_KEYCHAIN,
        } }
      : {}),
  },
}

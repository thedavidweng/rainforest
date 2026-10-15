import { expect, test } from "bun:test"
import { messagesIn, languages } from "./locales"

test("MyGo catalogs cover English and Simplified Chinese without changing region evidence", () => {
  expect(languages).toEqual(["en", "zh-Hans"])
  expect(messagesIn("zh-Hans").signInToAmazon()).toBe("登录 Amazon")
  expect(messagesIn("en").signInToAmazon()).toBe("Sign in to Amazon")
  expect(messagesIn("zh-Hans").experimental()).toContain("实验")
})

test("catalogs use locale-aware plural and number messages", () => {
  expect(messagesIn("en").photoCount(1)).toBe("1 photo")
  expect(messagesIn("en").photoCount(2)).toBe("2 photos")
  expect(messagesIn("zh-Hans").photoCount(2)).toBe("2 张照片")
  expect(messagesIn("en").quitMessage(2, 1)).toContain("2 transfers")
  expect(messagesIn("zh-Hans").quitMessage(2, 1)).toContain("2 个传输")
})

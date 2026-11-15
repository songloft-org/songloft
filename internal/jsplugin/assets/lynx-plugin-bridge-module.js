/**
 * `NativeModules.SongloftPluginBridge` for child <lynx-view> frames (Web target).
 *
 * This module runs inside the CHILD's worker. When the plugin SDK calls
 * NativeModules.SongloftPluginBridge.hostCall(...), it forwards through `call`
 * to the child's onNativeModulesCall, where lynx-frame-host.js intercepts it
 * and relays to the parent worker via sendGlobalEvent.
 */

export default function (_nativeModules, call) {
  return {
    registerChild(frameId) {
      void call('registerChild', [frameId])
    },

    registerHost(frameId) {
      void call('registerHost', [frameId])
    },

    unregisterHost(frameId) {
      void call('unregisterHost', [frameId])
    },

    hostCall(frameId, callId, ns, method, paramsJson) {
      void call('hostCall', [frameId, callId, ns, method, paramsJson])
    },

    hostReply(frameId, callId, resultJson) {
      void call('hostReply', [frameId, callId, resultJson])
    },

    pushToChild(frameId, eventName, dataJson) {
      void call('pushToChild', [frameId, eventName, dataJson])
    },
  }
}

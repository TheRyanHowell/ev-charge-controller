self.addEventListener('install', (event) => {
  self.skipWaiting()
})

self.addEventListener('activate', (event) => {
  event.waitUntil(clients.claim())
})

// pushSubscriptionExpiryThresholdMs is the window before expirationTime
// at which the subscription is considered expiring and should be refreshed.
const pushSubscriptionExpiryThresholdMs = 24 * 60 * 60 * 1000

// pushResubscribeCooldownMs is the minimum interval between resubscriptions.
// 24 hours prevents spamming resubscriptions on every notification.
const pushResubscribeCooldownMs = 24 * 60 * 60 * 1000

// pushLastResubscribeKey is the localStorage key for tracking last resubscribe time.
const pushLastResubscribeKey = 'evcc_lastPushResubscribe'

function getLastResubscribeTime() {
  try {
    return Number(localStorage.getItem(pushLastResubscribeKey)) || 0
  } catch {
    return 0
  }
}

function setLastResubscribeTime() {
  try {
    localStorage.setItem(pushLastResubscribeKey, String(Date.now()))
  } catch {
    // localStorage may be unavailable in some contexts
  }
}

function canResubscribe() {
  const last = getLastResubscribeTime()
  return Date.now() - last >= pushResubscribeCooldownMs
}

self.addEventListener('push', (event) => {
  const data = event.data ? event.data.json() : {}
  const title = data.title || 'EV Charge'
  const body = data.body || ''
  const vibration = data.vibration || [300, 100, 300, 100, 300, 100, 300, 100, 600, 200]

  event.waitUntil(
    self.registration.showNotification(title, {
      body,
      icon: '/favicon.ico',
      badge: '/favicon.ico',
      vibrate: vibration,
      requireInteraction: true,
      tag: title,
    }).then(() => {
      return maybeResubscribeToPush()
    })
  )
})

function isSubscriptionExpiring(subscription) {
  if (!subscription) {
    return false
  }
  if (subscription.expirationTime === null) {
    return true
  }
  const expiresInMs = subscription.expirationTime - Date.now()
  return expiresInMs < pushSubscriptionExpiryThresholdMs
}

function urlBase64ToBufferKey(base64String) {
  var padding = '='.repeat((4 - (base64String.length % 4)) % 4)
  var base64 = base64String.replace(/-/g, '+').replace(/_/g, '/') + padding
  var binaryString = atob(base64)
  var bytes = new Uint8Array(binaryString.length)
  for (var i = 0; i < binaryString.length; i++) {
    bytes[i] = binaryString.charCodeAt(i)
  }
  return bytes.buffer
}

function maybeResubscribeToPush() {
  return self.registration.pushManager.getSubscription().then(function (subscription) {
    if (!isSubscriptionExpiring(subscription) || !canResubscribe()) {
      return
    }
    return subscription.unsubscribe().then(function () {
      return fetch('/api/push-subscriptions').then(function (res) {
        return res.json()
      }).then(function (data) {
        var applicationServerKey = data.publicKey ? urlBase64ToBufferKey(data.publicKey) : undefined
        return self.registration.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey: applicationServerKey,
        })
      })
    }).then(function (newSubscription) {
      setLastResubscribeTime()
      return fetch('/api/push-subscriptions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          endpoint: newSubscription.endpoint,
          p256dh_key: newSubscription.toJSON().keys?.p256dh ?? '',
          auth_key: newSubscription.toJSON().keys?.auth ?? '',
        }),
      })
    }).catch(function (error) {
      console.error('[SW] Failed to resubscribe to push:', error)
    })
  })
}

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  event.waitUntil(
    clients.matchAll({ type: 'window' }).then((matches) => {
      if (matches.length > 0) {
        return matches[0].focus()
      }
      return clients.openWindow('/')
    }),
  )
})

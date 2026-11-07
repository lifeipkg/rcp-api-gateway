package com.risk.platform.gateway.auth;

/** Authorization 头解析：只认 Bearer 方案，缺失或格式不对一律返回 null。 */
public final class BearerToken {

    private static final String SCHEME = "Bearer ";

    private BearerToken() {
    }

    public static String extract(String authorizationHeader) {
        if (authorizationHeader == null) {
            return null;
        }
        String header = authorizationHeader.trim();
        if (header.length() <= SCHEME.length() || !header.regionMatches(true, 0, SCHEME, 0, SCHEME.length())) {
            return null;
        }
        String token = header.substring(SCHEME.length()).trim();
        return token.isEmpty() ? null : token;
    }
}

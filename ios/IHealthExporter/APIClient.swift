import Foundation

enum APIError: LocalizedError {
    case invalidURL, invalidResponse, server(Int, String)
    var errorDescription: String? {
        switch self {
        case .invalidURL: "Некорректный адрес сервера"
        case .invalidResponse: "Некорректный ответ сервера"
        case .server(let code, let body): "Сервер вернул HTTP \(code): \(body)"
        }
    }
}

struct APIClient {
    let baseURL: String
    let token: String

    func health() async throws {
        guard let url = URL(string: normalizedBaseURL + "/healthz") else { throw APIError.invalidURL }
        var request = URLRequest(url: url)
        request.timeoutInterval = 10
        let (_, response) = try await perform(request)
        guard let http = response as? HTTPURLResponse, http.statusCode == 200 else { throw APIError.invalidResponse }
    }

    func upload(_ batch: UploadBatch) async throws -> UploadResult {
        guard let url = URL(string: normalizedBaseURL + "/upload/v1/batches") else { throw APIError.invalidURL }
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.setValue("Bearer \(normalizedToken)", forHTTPHeaderField: "Authorization")
        request.timeoutInterval = 120
        request.httpBody = try JSONEncoder().encode(batch)
        let (data, response) = try await perform(request)
        guard let http = response as? HTTPURLResponse else { throw APIError.invalidResponse }
        guard http.statusCode == 200 else { throw APIError.server(http.statusCode, String(data: data, encoding: .utf8) ?? "") }
        return try JSONDecoder().decode(UploadResult.self, from: data)
    }

    private var normalizedBaseURL: String {
        baseURL.trimmingCharacters(in: .whitespacesAndNewlines.union(CharacterSet(charactersIn: "/")))
    }

    private var normalizedToken: String {
        token.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    private func perform(_ request: URLRequest) async throws -> (Data, URLResponse) {
        let retryable: Set<URLError.Code> = [.timedOut, .networkConnectionLost, .cannotConnectToHost, .notConnectedToInternet]
        var lastError: Error?
        for attempt in 0..<3 {
            do {
                return try await URLSession.shared.data(for: request)
            } catch let error as URLError where retryable.contains(error.code) {
                lastError = error
                if attempt < 2 {
                    try await Task.sleep(for: .seconds(Double(1 << attempt)))
                }
            }
        }
        throw lastError ?? APIError.invalidResponse
    }
}

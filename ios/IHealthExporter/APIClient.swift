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
        let (_, response) = try await URLSession.shared.data(for: request)
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
        let (data, response) = try await URLSession.shared.data(for: request)
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
}

package httpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	commonetcd "ecommerce/etcd"
)

var (
	ErrProductServiceUnavailable = errors.New("商品服务暂时不可用")
	ErrProductSKUNotFound        = errors.New("SKU不存在")
	ErrProductNotFound           = errors.New("商品不存在")
	errRetryableProductRequest   = errors.New("商品服务请求失败，可以重试")
)

const (
	productRequestTimeout  = 2 * time.Second
	productRetryCount      = 3
	productRetryInterval   = 300 * time.Millisecond
	maxProductResponseSize = 1 << 20
)

var productHTTPClient = &http.Client{Timeout: productRequestTimeout}

// ProductSKUDetail 是库存服务校验入库商品时需要的 SKU 信息。
type ProductSKUDetail struct {
	SKUID     int64   `json:"sku_id"`
	ProductID int64   `json:"product_id"`
	SKUName   string  `json:"sku_name"`
	SKUImage  *string `json:"sku_image"`
	Status    int8    `json:"status"`
}

// ProductDetail 是库存服务展示入库明细时需要的商品信息。
type ProductDetail struct {
	ProductID   int64  `json:"product_id"`
	ProductName string `json:"product_name"`
}

type productServiceResponse struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Data    *ProductSKUDetail `json:"data"`
}

type productDetailServiceResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    *ProductDetail `json:"data"`
}

// GetProductServiceAddresses 从 etcd 获取商品服务的全部可用实例。
func GetProductServiceAddresses() ([]string, error) {
	addresses, err := commonetcd.GetService("product-service")
	if err != nil || len(addresses) == 0 {
		return nil, ErrProductServiceUnavailable
	}
	return addresses, nil
}

// GetProductSKU 调用商品服务查询指定 SKU，并在临时故障时有限重试。
func GetProductSKU(ctx context.Context, skuID int64, authorization string) (*ProductSKUDetail, error) {
	addresses, err := GetProductServiceAddresses()
	if err != nil {
		return nil, err
	}

	for attempt := 0; attempt < productRetryCount; attempt++ {
		// 多实例场景下，每次重试轮换一个商品服务实例。
		address := addresses[attempt%len(addresses)]
		url := fmt.Sprintf("http://%s/api/v1/admin/skus/%d", address, skuID)

		sku, err := doProductGet(ctx, url, authorization)
		if err == nil {
			return sku, nil
		}
		if errors.Is(err, ErrProductSKUNotFound) {
			return nil, err
		}
		if !errors.Is(err, errRetryableProductRequest) {
			return nil, ErrProductServiceUnavailable
		}

		if attempt < productRetryCount-1 {
			if err := waitForProductRetry(ctx); err != nil {
				return nil, ErrProductServiceUnavailable
			}
		}
	}

	return nil, ErrProductServiceUnavailable
}

// GetProductDetail 调用商品服务查询指定商品，并在临时故障时有限重试。
func GetProductDetail(ctx context.Context, productID int64, authorization string) (*ProductDetail, error) {
	addresses, err := GetProductServiceAddresses()
	if err != nil {
		return nil, err
	}

	for attempt := 0; attempt < productRetryCount; attempt++ {
		address := addresses[attempt%len(addresses)]
		url := fmt.Sprintf("http://%s/api/v1/admin/products/%d", address, productID)

		product, err := doProductDetailGet(ctx, url, authorization)
		if err == nil {
			return product, nil
		}
		if errors.Is(err, ErrProductNotFound) {
			return nil, err
		}
		if !errors.Is(err, errRetryableProductRequest) {
			return nil, ErrProductServiceUnavailable
		}

		if attempt < productRetryCount-1 {
			if err := waitForProductRetry(ctx); err != nil {
				return nil, ErrProductServiceUnavailable
			}
		}
	}

	return nil, ErrProductServiceUnavailable
}

// doProductGet 发送一次 SKU 查询请求并解析商品服务的统一响应。
func doProductGet(ctx context.Context, url, authorization string) (*ProductSKUDetail, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, ErrProductServiceUnavailable
	}
	if authorization = strings.TrimSpace(authorization); authorization != "" {
		req.Header.Set("Authorization", authorization)
	}

	resp, err := productHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errRetryableProductRequest, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrProductSKUNotFound
	case resp.StatusCode >= http.StatusInternalServerError:
		return nil, fmt.Errorf("%w: HTTP %d", errRetryableProductRequest, resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return nil, ErrProductServiceUnavailable
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxProductResponseSize+1))
	if err != nil || len(body) > maxProductResponseSize {
		return nil, fmt.Errorf("%w: 响应读取失败", errRetryableProductRequest)
	}

	var result productServiceResponse
	if err := json.Unmarshal(body, &result); err != nil || result.Code != 0 || result.Data == nil {
		return nil, fmt.Errorf("%w: 响应格式错误", errRetryableProductRequest)
	}

	return result.Data, nil
}

// doProductDetailGet 发送一次商品详情查询请求并解析统一响应。
func doProductDetailGet(ctx context.Context, url, authorization string) (*ProductDetail, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, ErrProductServiceUnavailable
	}
	if authorization = strings.TrimSpace(authorization); authorization != "" {
		req.Header.Set("Authorization", authorization)
	}

	resp, err := productHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errRetryableProductRequest, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrProductNotFound
	case resp.StatusCode >= http.StatusInternalServerError:
		return nil, fmt.Errorf("%w: HTTP %d", errRetryableProductRequest, resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return nil, ErrProductServiceUnavailable
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxProductResponseSize+1))
	if err != nil || len(body) > maxProductResponseSize {
		return nil, fmt.Errorf("%w: 响应读取失败", errRetryableProductRequest)
	}

	var result productDetailServiceResponse
	if err := json.Unmarshal(body, &result); err != nil || result.Code != 0 || result.Data == nil {
		return nil, fmt.Errorf("%w: 响应格式错误", errRetryableProductRequest)
	}

	return result.Data, nil
}

func waitForProductRetry(ctx context.Context) error {
	timer := time.NewTimer(productRetryInterval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
